//go:build windows

package shell

import (
	"syscall"
	"unsafe"
)

// Message shows text to the user. The Windows build has no console (it is a
// GUI-subsystem program), so a native message box is the only way to be seen.
func Message(title, text string, isError bool) {
	const (
		mbOK        = 0x0
		mbIconError = 0x10
		mbIconInfo  = 0x40
	)
	flags := uintptr(mbOK | mbIconInfo)
	if isError {
		flags = mbOK | mbIconError
	}
	t, _ := syscall.UTF16PtrFromString(text)
	c, _ := syscall.UTF16PtrFromString(title)
	syscall.NewLazyDLL("user32.dll").NewProc("MessageBoxW").Call(0,
		uintptr(unsafe.Pointer(t)), uintptr(unsafe.Pointer(c)), flags)
}
