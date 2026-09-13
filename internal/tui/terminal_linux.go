//go:build linux

package tui

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"unsafe"
)

type winsize struct{ Row, Col, Xpixel, Ypixel uint16 }

type InputKind int

const (
	InputClick InputKind = iota
	InputUp
	InputDown
	InputPageUp
	InputPageDown
	InputHome
	InputEnd
)

type InputEvent struct {
	Kind InputKind
	X    int
	Y    int
}

type MouseEvent struct {
	X int
	Y int
}

func Size() (rows, cols int) {
	ws := winsize{}
	_, _, e := syscall.Syscall(syscall.SYS_IOCTL, os.Stdout.Fd(), uintptr(syscall.TIOCGWINSZ), uintptr(unsafe.Pointer(&ws)))
	if e != 0 || ws.Row == 0 || ws.Col == 0 {
		return 24, 80
	}
	return int(ws.Row), int(ws.Col)
}

func Enter() { os.Stdout.WriteString("\033[?1049h\033[?25l") }

func Leave() {
	os.Stdout.WriteString("\033[?1000l\033[?1006l\033[?25h\033[?1049l")
}

func Draw(s string) { os.Stdout.WriteString("\033[H\033[2J" + s) }

func setTermios(fd uintptr, term *syscall.Termios) syscall.Errno {
	_, _, e := syscall.Syscall(syscall.SYS_IOCTL, fd, uintptr(syscall.TCSETS), uintptr(unsafe.Pointer(term)))
	return e
}

// StartInput enables xterm button/wheel tracking and puts stdin into
// non-canonical mode. ISIG stays enabled, so Ctrl-C still reaches the signal
// handler. No pointer-motion mode is enabled: touch/drag gestures are left to
// the terminal emulator, while keyboard and wheel scrolling work directly.
func StartInput() (<-chan InputEvent, func()) {
	fd := os.Stdin.Fd()
	old := syscall.Termios{}
	_, _, e := syscall.Syscall(syscall.SYS_IOCTL, fd, uintptr(syscall.TCGETS), uintptr(unsafe.Pointer(&old)))
	if e != 0 {
		return nil, func() {}
	}

	raw := old
	raw.Lflag &^= syscall.ICANON | syscall.ECHO
	raw.Iflag &^= syscall.ICRNL | syscall.IXON
	raw.Cc[syscall.VMIN] = 1
	raw.Cc[syscall.VTIME] = 0
	if e := setTermios(fd, &raw); e != 0 {
		return nil, func() {}
	}

	os.Stdout.WriteString("\033[?1000h\033[?1006h")
	events := make(chan InputEvent, 16)
	go readInputEvents(events)

	var once sync.Once
	stop := func() {
		once.Do(func() {
			os.Stdout.WriteString("\033[?1000l\033[?1006l")
			_ = setTermios(fd, &old)
		})
	}
	return events, stop
}

func emitInput(out chan<- InputEvent, ev InputEvent) {
	select {
	case out <- ev:
	default:
	}
}

func readInputEvents(out chan<- InputEvent) {
	defer close(out)
	r := bufio.NewReader(os.Stdin)
	for {
		b, err := r.ReadByte()
		if err != nil {
			return
		}
		if b != '\x1b' {
			continue
		}
		b, err = r.ReadByte()
		if err != nil {
			return
		}
		if b == 'O' {
			if code, err := r.ReadByte(); err == nil {
				switch code {
				case 'H':
					emitInput(out, InputEvent{Kind: InputHome})
				case 'F':
					emitInput(out, InputEvent{Kind: InputEnd})
				}
			}
			continue
		}
		if b != '[' {
			continue
		}
		b, err = r.ReadByte()
		if err != nil {
			return
		}
		switch b {
		case 'A':
			emitInput(out, InputEvent{Kind: InputUp})
			continue
		case 'B':
			emitInput(out, InputEvent{Kind: InputDown})
			continue
		case 'H':
			emitInput(out, InputEvent{Kind: InputHome})
			continue
		case 'F':
			emitInput(out, InputEvent{Kind: InputEnd})
			continue
		case '5', '6':
			if term, err := r.ReadByte(); err == nil && term == '~' {
				if b == '5' {
					emitInput(out, InputEvent{Kind: InputPageUp})
				} else {
					emitInput(out, InputEvent{Kind: InputPageDown})
				}
			}
			continue
		case '<':
			// SGR mouse sequence: <button;x;yM (press/wheel), ...m release.
		default:
			continue
		}

		var payload strings.Builder
		for payload.Len() < 48 {
			b, err = r.ReadByte()
			if err != nil {
				return
			}
			if b == 'M' || b == 'm' {
				if b == 'M' {
					if ev, ok := parseSGRInput(payload.String()); ok {
						emitInput(out, ev)
					}
				}
				break
			}
			if (b < '0' || b > '9') && b != ';' {
				break
			}
			payload.WriteByte(b)
		}
	}
}

func parseSGRInput(payload string) (InputEvent, bool) {
	parts := strings.Split(payload, ";")
	if len(parts) != 3 {
		return InputEvent{}, false
	}
	button, err1 := strconv.Atoi(parts[0])
	x, err2 := strconv.Atoi(parts[1])
	y, err3 := strconv.Atoi(parts[2])
	if err1 != nil || err2 != nil || err3 != nil || x < 1 || y < 1 || button&32 != 0 {
		return InputEvent{}, false
	}
	if button&64 != 0 {
		switch button & 3 {
		case 0:
			return InputEvent{Kind: InputUp, X: x, Y: y}, true
		case 1:
			return InputEvent{Kind: InputDown, X: x, Y: y}, true
		default:
			return InputEvent{}, false
		}
	}
	if button&3 != 0 {
		return InputEvent{}, false
	}
	return InputEvent{Kind: InputClick, X: x, Y: y}, true
}

func parseSGRMouse(payload string) (MouseEvent, bool) {
	ev, ok := parseSGRInput(payload)
	if !ok || ev.Kind != InputClick {
		return MouseEvent{}, false
	}
	return MouseEvent{X: ev.X, Y: ev.Y}, true
}
