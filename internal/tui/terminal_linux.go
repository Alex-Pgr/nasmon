//go:build linux

package tui

import (
	"os"
	"syscall"
	"unsafe"
)

type winsize struct{ Row, Col, Xpixel, Ypixel uint16 }

func Size() (rows, cols int) {
	ws := winsize{}
	_, _, e := syscall.Syscall(syscall.SYS_IOCTL, os.Stdout.Fd(), uintptr(syscall.TIOCGWINSZ), uintptr(unsafe.Pointer(&ws)))
	if e != 0 || ws.Row == 0 || ws.Col == 0 {
		return 24, 80
	}
	return int(ws.Row), int(ws.Col)
}
func Enter()        { os.Stdout.WriteString("\033[?1049h\033[?25l") }
func Leave()        { os.Stdout.WriteString("\033[?25h\033[?1049l") }
func Draw(s string) { os.Stdout.WriteString("\033[H\033[2J" + s) }
