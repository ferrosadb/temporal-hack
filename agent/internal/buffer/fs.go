package buffer

import (
	"errors"
	"os"
)

var errExists = errors.New("dir exists")

func mkdirAll(p string) error {
	if err := os.MkdirAll(p, 0o755); err != nil {
		return err
	}
	return nil
}
