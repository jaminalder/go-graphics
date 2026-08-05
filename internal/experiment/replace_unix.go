//go:build !windows

package experiment

import "os"

func replaceFile(source, destination string) error {
	return os.Rename(source, destination)
}
