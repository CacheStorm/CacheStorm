//go:build !windows

package persistence

import "os"

func openAOFFile(path string, flags int, mode os.FileMode) (*os.File, error) {
	return os.OpenFile(path, flags, mode)
}
