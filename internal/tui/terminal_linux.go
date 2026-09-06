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

// StartMouseInput enables xterm button tracking with SGR coordinates and puts
// stdin into non-canonical mode. ISIG is intentionally kept, so Ctrl-C still
// reaches the existing signal handler. If stdin is not a TTY, mouse input is
// simply unavailable and nasmon continues as a read-only TUI.
func StartMouseInput() (<-chan MouseEvent, func()) {
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
	events := make(chan MouseEvent, 8)
	go readMouseEvents(events)

	var once sync.Once
	stop := func() {
		once.Do(func() {
			os.Stdout.WriteString("\033[?1000l\033[?1006l")
			_ = setTermios(fd, &old)
		})
	}
	return events, stop
}

func readMouseEvents(out chan<- MouseEvent) {
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
		if err != nil || b != '[' {
			continue
		}
		b, err = r.ReadByte()
		if err != nil || b != '<' {
			continue
		}

		var payload strings.Builder
		for payload.Len() < 48 {
			b, err = r.ReadByte()
			if err != nil {
				return
			}
			if b == 'M' || b == 'm' {
				// SGR uses M for press and m for release. Only a left-button
				// press is useful for sortable headers.
				if b == 'M' {
					if ev, ok := parseSGRMouse(payload.String()); ok {
						select {
						case out <- ev:
						default:
						}
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

func parseSGRMouse(payload string) (MouseEvent, bool) {
	parts := strings.Split(payload, ";")
	if len(parts) != 3 {
		return MouseEvent{}, false
	}
	button, err1 := strconv.Atoi(parts[0])
	x, err2 := strconv.Atoi(parts[1])
	y, err3 := strconv.Atoi(parts[2])
	if err1 != nil || err2 != nil || err3 != nil || x < 1 || y < 1 {
		return MouseEvent{}, false
	}
	// Ignore motion, wheel and non-left buttons.
	if button&32 != 0 || button&64 != 0 || button&3 != 0 {
		return MouseEvent{}, false
	}
	return MouseEvent{X: x, Y: y}, true
}
