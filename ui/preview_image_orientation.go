package ui

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"io"
)

// jpegOrientationSwapsAxes reads JPEG header segments up to the first EXIF
// directory or the start of the scan, without decoding the bitmap.
func jpegOrientationSwapsAxes(data io.Reader) bool {
	reader := bufio.NewReader(data)
	var pair [2]byte
	if _, err := io.ReadFull(reader, pair[:]); err != nil || pair != [2]byte{0xff, 0xd8} {
		return false
	}
	for {
		marker, err := reader.ReadByte()
		if err != nil || marker != 0xff {
			return false
		}
		for marker == 0xff {
			marker, err = reader.ReadByte()
			if err != nil {
				return false
			}
		}
		switch marker {
		case 0xda, 0xd9: // Start of scan, end of image.
			return false
		case 0x01: // Standalone TEM marker.
			continue
		}
		if _, err := io.ReadFull(reader, pair[:]); err != nil {
			return false
		}
		size := int(binary.BigEndian.Uint16(pair[:])) - len(pair)
		if size < 0 {
			return false
		}
		if marker != 0xe1 {
			if _, err := reader.Discard(size); err != nil {
				return false
			}
			continue
		}
		segment := make([]byte, size)
		if _, err := io.ReadFull(reader, segment); err != nil {
			return false
		}
		if tiff, ok := bytes.CutPrefix(segment, []byte("Exif\x00\x00")); ok {
			return exifOrientationSwapsAxes(tiff)
		}
	}
}

func exifOrientationSwapsAxes(tiff []byte) bool {
	if len(tiff) < 8 {
		return false
	}
	var order binary.ByteOrder
	switch string(tiff[:2]) {
	case "II":
		order = binary.LittleEndian
	case "MM":
		order = binary.BigEndian
	default:
		return false
	}
	offset := order.Uint32(tiff[4:8])
	if order.Uint16(tiff[2:4]) != 42 || offset < 8 || offset > uint32(len(tiff)-2) {
		return false
	}
	directory := tiff[offset:]
	count, entries := int(order.Uint16(directory)), directory[2:]
	const entrySize = 12
	for i := range min(count, len(entries)/entrySize) {
		entry := entries[i*entrySize:][:entrySize]
		if order.Uint16(entry) == 0x0112 {
			// Orientation is one TIFF SHORT, stored in the entry's value.
			if order.Uint16(entry[2:]) != 3 || order.Uint32(entry[4:]) != 1 {
				return false
			}
			value := order.Uint16(entry[8:])
			return value >= 5 && value <= 8
		}
	}
	return false
}
