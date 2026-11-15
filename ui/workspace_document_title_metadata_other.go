//go:build !darwin

package ui

import "errors"

const documentTitleMetadataSupported = false

func readDocumentTitleMetadata(string) ([]string, bool, error) {
	return nil, false, errors.ErrUnsupported
}

func writeDocumentTitleTags(string, []string) error {
	return errors.ErrUnsupported
}

func writeDocumentTitleLock(string, bool) error {
	return errors.ErrUnsupported
}
