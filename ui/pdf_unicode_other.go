//go:build !darwin

package ui

func pdfUnicodeReplacements(map[string][]uint16) map[string]map[uint16]uint16 {
	return nil
}
