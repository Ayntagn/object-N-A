package jpegscale

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image/jpeg"
	"io"
)

const (
	markerSOF0 = 0xc0
	markerSOF1 = 0xc1
	markerSOF2 = 0xc2
	markerRST0 = 0xd0
	markerRST7 = 0xd7
	markerSOI  = 0xd8
	markerEOI  = 0xd9
	markerSOS  = 0xda
)

// inspectFrame finds the actual image terminator, bounds the validation decode
// and checks scan coverage. image/jpeg validates the entropy-coded data but
// can complete with some frame components still absent.
func inspectFrame(data []byte, limit uint64) (width, height, end int, err error) {
	if len(data) < 2 {
		return 0, 0, 0, io.ErrUnexpectedEOF
	}
	if data[0] != 0xff || data[1] != markerSOI {
		return 0, 0, 0, jpeg.FormatError("missing SOI marker")
	}

	var scans scanCoverage
	for pos := 2; pos < len(data); {
		next := bytes.IndexByte(data[pos:], 0xff)
		if next < 0 {
			break
		}
		pos += next + 1
		for pos < len(data) && data[pos] == 0xff {
			pos++
		}
		if pos == len(data) {
			break
		}
		marker := data[pos]
		pos++
		switch {
		case marker == 0 || markerRST0 <= marker && marker <= markerRST7:
			// Stuffed entropy bytes and restart markers have no segment payload.
			continue
		case marker == markerEOI:
			if scans.numComponents == 0 || scans.covered != (1<<scans.numComponents)-1 {
				return 0, 0, 0, jpeg.FormatError("image is missing one or more component scans")
			}
			return width, height, pos, nil
		case marker == markerSOI:
			return 0, 0, 0, jpeg.FormatError("unexpected SOI marker")
		case marker == 0x01: // TEM
			continue
		}

		if len(data)-pos < 2 {
			break
		}
		length := int(binary.BigEndian.Uint16(data[pos:]))
		if length < 2 {
			return 0, 0, 0, jpeg.FormatError("short marker segment")
		}
		if length > len(data)-pos {
			break
		}
		payload := data[pos+2 : pos+length]
		pos += length
		switch marker {
		case markerSOF0, markerSOF1, markerSOF2:
			if scans.numComponents != 0 {
				return 0, 0, 0, jpeg.FormatError("multiple SOF markers")
			}
			width, height, err = scans.frameHeader(payload, marker == markerSOF2)
			if err != nil {
				return 0, 0, 0, err
			}
			if uint64(width)*uint64(height) > limit {
				return 0, 0, 0, fmt.Errorf("%w: %dx%d over %d pixels", ErrImageTooLarge, width, height, limit)
			}
		case markerSOS:
			if err := scans.scanHeader(payload); err != nil {
				return 0, 0, 0, err
			}
		default:
			if marker >= 0xc0 && marker <= 0xcf && marker != 0xc4 {
				return 0, 0, 0, jpeg.UnsupportedError("JPEG frame or coding marker")
			}
		}
	}
	return 0, 0, 0, io.ErrUnexpectedEOF
}

type scanCoverage struct {
	components    [4]byte
	numComponents int
	covered       uint
	progressive   bool

	// In progressive mode each entry is the last Al plus one, with zero
	// denoting an unseen coefficient.
	levels [4][64]uint8
}

func (s *scanCoverage) frameHeader(payload []byte, progressive bool) (int, int, error) {
	if len(payload) < 6 {
		return 0, 0, jpeg.FormatError("short frame header")
	}
	count := int(payload[5])
	if count != 1 && count != 3 && count != 4 {
		return 0, 0, jpeg.UnsupportedError("number of components")
	}
	if len(payload) != 6+3*count {
		return 0, 0, jpeg.FormatError("invalid frame header length")
	}
	height := int(binary.BigEndian.Uint16(payload[1:]))
	width := int(binary.BigEndian.Uint16(payload[3:]))
	if height == 0 || width == 0 {
		return 0, 0, jpeg.FormatError("zero image dimension")
	}
	for i := range count {
		id := payload[6+3*i]
		if bytes.IndexByte(s.components[:i], id) >= 0 {
			return 0, 0, jpeg.FormatError("duplicate frame component")
		}
		s.components[i] = id
	}
	s.numComponents = count
	s.progressive = progressive
	return width, height, nil
}

func (s *scanCoverage) scanHeader(payload []byte) error {
	if len(payload) == 0 {
		return jpeg.FormatError("empty scan header")
	}
	count := int(payload[0])
	if count == 0 || count > s.numComponents || len(payload) != 4+2*count {
		return jpeg.FormatError("invalid scan header length or component count")
	}
	start, end, shift := payload[1+2*count], payload[2+2*count], payload[3+2*count]
	high, low := shift>>4, shift&15
	if s.progressive {
		if start > end || end >= 64 || (start == 0 && end != 0) || (start != 0 && count != 1) ||
			high > 13 || low > 13 || high != 0 && high != low+1 {
			return jpeg.FormatError("invalid progressive scan parameters")
		}
	} else if start != 0 || end != 63 || shift != 0 {
		return jpeg.FormatError("invalid sequential scan parameters")
	}

	var inScan uint
	for i := range count {
		index := bytes.IndexByte(s.components[:s.numComponents], payload[1+2*i])
		if index < 0 || inScan&(1<<index) != 0 {
			return jpeg.FormatError("invalid scan component selector")
		}
		mask := uint(1) << index
		inScan |= mask
		if s.progressive {
			if start != 0 && s.covered&mask == 0 {
				return jpeg.FormatError("AC scan precedes its DC scan")
			}
			for k := start; k <= end; k++ {
				prev := s.levels[index][k]
				if high == 0 && prev != 0 || high != 0 && prev != high+1 {
					return jpeg.FormatError("invalid progressive scan order")
				}
				s.levels[index][k] = low + 1
			}
		} else if s.covered&mask != 0 {
			return jpeg.FormatError("repeated sequential component scan")
		}
		if start == 0 {
			s.covered |= mask
		}
	}
	return nil
}
