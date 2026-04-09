//go:build windows

package main

import "syscall"

func hideConsoleWindow() {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	getConsoleWindow := kernel32.NewProc("GetConsoleWindow")
	hwnd, _, _ := getConsoleWindow.Call()
	if hwnd == 0 {
		return
	}

	user32 := syscall.NewLazyDLL("user32.dll")
	showWindow := user32.NewProc("ShowWindow")
	const swHide = 0
	_, _, _ = showWindow.Call(hwnd, uintptr(swHide))
}
