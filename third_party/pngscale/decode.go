// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pngscale

import (
	"bufio"
	"compress/zlib"
	"encoding/binary"
	"hash/crc32"
	"image/color"
	"io"
	"math"
)

// FormatError reports that the input cannot be decoded as a PNG.
type FormatError string

func (e FormatError) Error() string { return "pngscale: invalid PNG: " + string(e) }

const pngSignature = "\x89PNG\r\n\x1a\n"

type scanState struct {
	width, height, depth, colorType          int
	interlaced                               bool
	palette                                  [256]color.NRGBA
	paletteLen                               int
	transparent                              bool
	transparentR, transparentG, transparentB uint16
}

func (s *scanState) bitsPerPixel() int {
	switch s.colorType {
	case 0: // Grayscale
		if s.depth == 1 || s.depth == 2 || s.depth == 4 || s.depth == 8 || s.depth == 16 {
			return s.depth
		}
	case 3: // Indexed color
		if s.depth == 1 || s.depth == 2 || s.depth == 4 || s.depth == 8 {
			return s.depth
		}
	case 2, 4, 6: // RGB, grayscale with alpha, RGBA
		if s.depth == 8 || s.depth == 16 {
			switch s.colorType {
			case 2:
				return 3 * s.depth
			case 4:
				return 2 * s.depth
			case 6:
				return 4 * s.depth
			}
		}
	}
	return 0
}

type scanPass struct {
	x0, y0 int
	dx, dy int
}

// The Adam7 pass coordinates are in original image pixels.
var adam7Passes = [...]scanPass{
	{0, 0, 8, 8},
	{4, 0, 8, 8},
	{0, 4, 4, 8},
	{2, 0, 4, 4},
	{0, 2, 2, 4},
	{1, 0, 2, 2},
	{0, 1, 1, 2},
}

// scanDecoder tracks one PNG chunk at a time. The same reader presents the
// consecutive IDAT payloads as a single byte stream to zlib.
type scanDecoder struct {
	r         io.Reader
	remaining int
	crc       uint32
	header    [8]byte
	scratch   [4096]byte
}

// decodeScanlines calls setup after the image metadata is checked, then visits
// each unfiltered row of each nonempty pass. The row omits its filter byte and
// is valid until the next visit. Success includes validation through IEND.
func decodeScanlines(r io.Reader, setup func(scanState) error, visit func(row []byte, x0, y, dx int) error) error {
	d := scanDecoder{r: r}
	var s scanState
	if err := d.readMetadata(&s); err != nil {
		return err
	}
	if err := setup(s); err != nil {
		return err
	}
	if err := d.readScanlines(&s, visit); err != nil {
		return err
	}
	return d.finishImage()
}

func (d *scanDecoder) readMetadata(s *scanState) error {
	if err := readExact(d.r, d.header[:]); err != nil {
		return err
	}
	if string(d.header[:]) != pngSignature {
		return FormatError("invalid signature")
	}
	if err := d.nextChunk(); err != nil {
		return err
	}
	if d.chunkType() != "IHDR" {
		return FormatError("missing IHDR")
	}
	if err := d.readIHDR(s); err != nil {
		return err
	}

	seenPLTE, seenTRNS := false, false
	for {
		if err := d.nextChunk(); err != nil {
			return err
		}
		switch d.chunkType() {
		case "IDAT":
			if s.colorType == 3 && !seenPLTE {
				return FormatError("indexed color requires PLTE")
			}
			return nil
		case "PLTE":
			if seenPLTE || seenTRNS {
				return FormatError("PLTE out of order")
			}
			if err := d.readPLTE(s); err != nil {
				return err
			}
			seenPLTE = true
		case "tRNS":
			if seenTRNS || (s.colorType == 3 && !seenPLTE) {
				return FormatError("tRNS out of order")
			}
			if err := d.readTRNS(s); err != nil {
				return err
			}
			seenTRNS = true
		case "IHDR", "IEND":
			return FormatError("chunk out of order")
		default:
			if err := d.skipAncillary(); err != nil {
				return err
			}
		}
	}
}

func (d *scanDecoder) readIHDR(s *scanState) error {
	if d.remaining != 13 {
		return FormatError("invalid IHDR length")
	}
	p, err := d.readSmallChunk()
	if err != nil {
		return err
	}
	w, h := binary.BigEndian.Uint32(p[:4]), binary.BigEndian.Uint32(p[4:8])
	if w == 0 || h == 0 || w > math.MaxInt32 || h > math.MaxInt32 {
		return FormatError("invalid dimensions")
	}
	s.width, s.height = int(w), int(h)
	s.depth, s.colorType = int(p[8]), int(p[9])
	if s.bitsPerPixel() == 0 {
		return FormatError("invalid color type or bit depth")
	}
	if p[10] != 0 || p[11] != 0 || p[12] > 1 {
		return FormatError("invalid compression, filter, or interlace method")
	}
	s.interlaced = p[12] == 1
	return nil
}

func (d *scanDecoder) readPLTE(s *scanState) error {
	if s.colorType == 0 || s.colorType == 4 {
		return FormatError("PLTE forbidden for grayscale")
	}
	n := d.remaining / 3
	if d.remaining%3 != 0 || n == 0 || n > len(s.palette) || (s.colorType == 3 && n > 1<<s.depth) {
		return FormatError("invalid PLTE length")
	}
	p, err := d.readSmallChunk()
	if err != nil {
		return err
	}
	if s.colorType == 3 {
		s.paletteLen = n
		for i := range s.palette[:n] {
			s.palette[i] = color.NRGBA{R: p[3*i], G: p[3*i+1], B: p[3*i+2], A: 255}
		}
	}
	return nil
}

func (d *scanDecoder) readTRNS(s *scanState) error {
	switch s.colorType {
	case 0:
		if d.remaining != 2 {
			return FormatError("invalid grayscale tRNS length")
		}
	case 2:
		if d.remaining != 6 {
			return FormatError("invalid RGB tRNS length")
		}
	case 3:
		if d.remaining > s.paletteLen {
			return FormatError("too many palette alpha values")
		}
	default:
		return FormatError("tRNS forbidden with alpha channel")
	}
	p, err := d.readSmallChunk()
	if err != nil {
		return err
	}
	if s.colorType == 3 {
		for i, a := range p {
			s.palette[i].A = a
		}
		return nil
	}

	mask := uint16((uint32(1) << s.depth) - 1)
	s.transparent = true
	s.transparentR = binary.BigEndian.Uint16(p[:2]) & mask
	if s.colorType == 2 {
		s.transparentG = binary.BigEndian.Uint16(p[2:4]) & mask
		s.transparentB = binary.BigEndian.Uint16(p[4:6]) & mask
	}
	return nil
}

func (d *scanDecoder) nextChunk() error {
	if err := readExact(d.r, d.header[:]); err != nil {
		return err
	}
	length := binary.BigEndian.Uint32(d.header[:4])
	if length > math.MaxInt32 {
		return FormatError("invalid chunk length")
	}
	for _, c := range d.header[4:] {
		if lower := c | 0x20; lower < 'a' || lower > 'z' {
			return FormatError("invalid chunk type")
		}
	}
	d.remaining = int(length)
	d.crc = crc32.ChecksumIEEE(d.header[4:])
	return nil
}

func (d *scanDecoder) chunkType() string { return string(d.header[4:]) }

func (d *scanDecoder) verifyCRC() error {
	if err := readExact(d.r, d.header[:4]); err != nil {
		return err
	}
	if binary.BigEndian.Uint32(d.header[:4]) != d.crc {
		return FormatError("invalid chunk CRC")
	}
	return nil
}

func (d *scanDecoder) readPayload(p []byte) error {
	if err := readExact(d.r, p); err != nil {
		return err
	}
	d.remaining -= len(p)
	d.crc = crc32.Update(d.crc, crc32.IEEETable, p)
	return nil
}

// Only chunks with their length already checked against a fixed maximum are
// read whole. The returned slice is borrowed from scratch.
func (d *scanDecoder) readSmallChunk() ([]byte, error) {
	p := d.scratch[:d.remaining]
	if err := d.readPayload(p); err != nil {
		return nil, err
	}
	return p, d.verifyCRC()
}

func (d *scanDecoder) skipChunk() error {
	for d.remaining > 0 {
		if err := d.readPayload(d.scratch[:min(d.remaining, len(d.scratch))]); err != nil {
			return err
		}
	}
	return d.verifyCRC()
}

func (d *scanDecoder) skipAncillary() error {
	if d.header[4]&0x20 == 0 {
		return FormatError("unknown critical chunk " + d.chunkType())
	}
	return d.skipChunk()
}

// Read never exposes the first non-IDAT chunk to zlib. It preserves that
// chunk's header so finishImage can verify it after decompression.
func (d *scanDecoder) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	for d.chunkType() == "IDAT" {
		if d.remaining > 0 {
			n, err := d.r.Read(p[:min(len(p), d.remaining)])
			d.remaining -= n
			d.crc = crc32.Update(d.crc, crc32.IEEETable, p[:n])
			if err == io.EOF {
				err = io.ErrUnexpectedEOF
			}
			return n, err
		}
		if err := d.verifyCRC(); err != nil {
			return 0, err
		}
		if err := d.nextChunk(); err != nil {
			return 0, err
		}
	}
	return 0, io.EOF
}

func (d *scanDecoder) readScanlines(s *scanState, visit func(row []byte, x0, y, dx int) error) error {
	input := bufio.NewReader(d)
	header, err := input.Peek(2)
	if err == io.EOF {
		return io.ErrUnexpectedEOF
	}
	if err != nil {
		return err
	}
	if header[1]&0x20 != 0 {
		return FormatError("preset compression dictionary")
	}
	z, err := zlib.NewReader(input)
	if err != nil {
		return err
	}
	defer z.Close()

	bits := s.bitsPerPixel()
	rowLen := scanlineSize(s.width, bits)
	if rowLen > math.MaxInt/2 {
		return FormatError("scanline size overflows int")
	}
	rows := make([]byte, 2*int(rowLen))
	current, previous := rows[:int(rowLen)], rows[int(rowLen):]
	if s.interlaced {
		for _, pass := range adam7Passes {
			if err := readPass(z, s, pass, bits, current, previous, visit); err != nil {
				return err
			}
		}
	} else if err := readPass(z, s, scanPass{dx: 1, dy: 1}, bits, current, previous, visit); err != nil {
		return err
	}

	// Reading through EOF verifies the zlib Adler-32 checksum even when the
	// final row consumed precisely the decompressed bytes we expected.
	_, err = io.ReadFull(z, d.scratch[:1])
	if err == nil {
		return FormatError("too much scanline data")
	}
	if err == io.EOF {
		return nil
	}
	return err
}

func scanlineSize(width, bitsPerPixel int) int64 {
	return 1 + (int64(width)*int64(bitsPerPixel)+7)/8
}

func readPass(r io.Reader, s *scanState, pass scanPass, bits int, current, previous []byte,
	visit func(row []byte, x0, y, dx int) error) error {
	if s.width <= pass.x0 || s.height <= pass.y0 {
		return nil
	}
	width := 1 + (s.width-1-pass.x0)/pass.dx
	height := 1 + (s.height-1-pass.y0)/pass.dy
	rowLen := int(scanlineSize(width, bits))
	current, previous = current[:rowLen], previous[:rowLen]
	clear(previous[1:])
	filterStride := (bits + 7) / 8
	for i := 0; i < height; i++ {
		if err := readExact(r, current); err != nil {
			if err == io.ErrUnexpectedEOF {
				return FormatError("not enough scanline data")
			}
			return err
		}
		if err := unfilter(current[0], current[1:], previous[1:], filterStride); err != nil {
			return err
		}
		if err := visit(current[1:], pass.x0, pass.y0+i*pass.dy, pass.dx); err != nil {
			return err
		}
		current, previous = previous, current
	}
	return nil
}

func (d *scanDecoder) finishImage() error {
	endedIDAT := false
	for {
		switch d.chunkType() {
		case "IDAT":
			if endedIDAT {
				return FormatError("nonconsecutive IDAT")
			}
			// PNG permits unused bytes after the zlib stream in the last
			// IDAT. Check the CRCs of the entire consecutive IDAT sequence.
			if err := d.skipChunk(); err != nil {
				return err
			}
		case "IEND":
			if d.remaining != 0 {
				return FormatError("invalid IEND length")
			}
			return d.verifyCRC()
		case "IHDR", "PLTE", "tRNS":
			return FormatError("chunk out of order")
		default:
			endedIDAT = true
			if err := d.skipAncillary(); err != nil {
				return err
			}
		}
		if err := d.nextChunk(); err != nil {
			return err
		}
	}
}

func readExact(r io.Reader, p []byte) error {
	_, err := io.ReadFull(r, p)
	if err == io.EOF {
		return io.ErrUnexpectedEOF
	}
	return err
}
