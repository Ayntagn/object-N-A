//go:build !darwin && !linux && !windows

package workspace

import (
	"errors"
	"os"
)

func renameEntry(_ *os.File, _ string, _ *os.File, _ string) error {
	return errors.ErrUnsupported
}
