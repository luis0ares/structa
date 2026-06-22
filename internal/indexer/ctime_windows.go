//go:build windows

package indexer

import (
	"os"
	"syscall"
)

// creationTime returns the folder's creation time in seconds since the Unix
// epoch. On Windows this reads the real birth time from the file attributes.
func creationTime(info os.FileInfo) float64 {
	if d, ok := info.Sys().(*syscall.Win32FileAttributeData); ok {
		return float64(d.CreationTime.Nanoseconds()) / 1e9
	}
	return float64(info.ModTime().UnixNano()) / 1e9
}
