//go:build windows

package main

import (
	"syscall"
	"unsafe"
)

func initConsole() {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	setConsoleOutputCP := kernel32.NewProc("SetConsoleOutputCP")
	setConsoleCP := kernel32.NewProc("SetConsoleCP")

	if setConsoleOutputCP.Find() == nil {
		_, _, _ = setConsoleOutputCP.Call(65001) // Codepage 65001: UTF-8 nativo
	}
	if setConsoleCP.Find() == nil {
		_, _, _ = setConsoleCP.Call(65001)
	}

	getStdHandle := kernel32.NewProc("GetStdHandle")
	getConsoleMode := kernel32.NewProc("GetConsoleMode")
	setConsoleMode := kernel32.NewProc("SetConsoleMode")

	if setConsoleMode.Find() == nil && getConsoleMode.Find() == nil {
		handle, _, _ := getStdHandle.Call(uintptr(0xFFFFFFF5)) // STD_OUTPUT_HANDLE (-11)
		var mode uint32
		_, _, _ = getConsoleMode.Call(handle, uintptr(unsafe.Pointer(&mode)))
		mode |= 0x0004 // ENABLE_VIRTUAL_TERMINAL_PROCESSING (ANSI & emojis)
		_, _, _ = setConsoleMode.Call(handle, uintptr(mode))
	}
}
