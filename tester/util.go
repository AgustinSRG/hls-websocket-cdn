// Utils

package main

import (
	"errors"
	"os"
)

func CheckFileExists(file string) bool {
	if _, err := os.Stat(file); err == nil {
		return true
	} else if errors.Is(err, os.ErrNotExist) {
		// path/to/whatever does *not* exist
		return false
	} else {
		return false
	}
}
