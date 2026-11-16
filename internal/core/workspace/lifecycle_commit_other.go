//go:build !windows

package workspace

import "errors"

func unsupportedLifecycleCommit(err error) bool {
	return errors.Is(err, errors.ErrUnsupported)
}
