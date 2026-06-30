//go:build windows

package main

import (
	"os"

	"golang.org/x/sys/windows"
	"golang.org/x/term"
)

const utf8CodePage = 65001

func enableConsoleColor() {
	enableConsoleUTF8(os.Stdout)
	enableConsoleUTF8(os.Stderr)
}

func enableConsoleUTF8(f *os.File) {
	if f == nil || !term.IsTerminal(int(f.Fd())) {
		return
	}
	enableUTF8CodePage()
	enableVirtualTerminal(f)
}

func enableUTF8CodePage() {
	kernel32 := windows.NewLazySystemDLL("kernel32.dll")
	setConsoleOutputCP := kernel32.NewProc("SetConsoleOutputCP")
	setConsoleCP := kernel32.NewProc("SetConsoleCP")
	_, _, _ = setConsoleOutputCP.Call(utf8CodePage)
	_, _, _ = setConsoleCP.Call(utf8CodePage)
}

func enableVirtualTerminal(f *os.File) {
	handle := windows.Handle(f.Fd())
	var mode uint32
	if err := windows.GetConsoleMode(handle, &mode); err != nil {
		return
	}
	_ = windows.SetConsoleMode(handle, mode|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING)
}
