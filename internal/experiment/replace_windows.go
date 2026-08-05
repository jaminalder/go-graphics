//go:build windows

package experiment

import (
	"fmt"
	"syscall"
	"unsafe"
)

const (
	moveFileReplaceExisting = 0x1
	moveFileWriteThrough    = 0x8
)

var moveFileExW = syscall.NewLazyDLL("kernel32.dll").NewProc("MoveFileExW")

func replaceFile(source, destination string) error {
	sourcePtr, err := syscall.UTF16PtrFromString(source)
	if err != nil {
		return fmt.Errorf("encode replacement source: %w", err)
	}
	destinationPtr, err := syscall.UTF16PtrFromString(destination)
	if err != nil {
		return fmt.Errorf("encode replacement destination: %w", err)
	}
	succeeded, _, callErr := moveFileExW.Call(
		uintptr(unsafe.Pointer(sourcePtr)),
		uintptr(unsafe.Pointer(destinationPtr)),
		moveFileReplaceExisting|moveFileWriteThrough,
	)
	if succeeded == 0 {
		return callErr
	}
	return nil
}
