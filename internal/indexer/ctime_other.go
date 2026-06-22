//go:build !windows

package indexer

import "os"

// creationTime returns a best-effort creation time. Go exposes no portable
// birth-time API, so on non-Windows platforms we fall back to the modification
// time. (Production builds target Windows, where the real ctime is available.)
func creationTime(info os.FileInfo) float64 {
	return float64(info.ModTime().UnixNano()) / 1e9
}
