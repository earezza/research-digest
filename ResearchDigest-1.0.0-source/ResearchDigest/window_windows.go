package main

import (
	"os/exec"
	"syscall"
	"unsafe"
)

// hideWindow keeps helper programs (opening the browser) from flashing a console window.
func hideWindow(cmd *exec.Cmd) { cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true} }

// showError shows a message box, since the program runs without a terminal window.
func showError(msg string) {
	user32 := syscall.NewLazyDLL("user32.dll")
	box := user32.NewProc("MessageBoxW")
	text, _ := syscall.UTF16PtrFromString(msg)
	title, _ := syscall.UTF16PtrFromString("Research Digest")
	_, _, _ = box.Call(0, uintptr(unsafe.Pointer(text)), uintptr(unsafe.Pointer(title)), 0x10) // MB_ICONERROR
}
