// Package pngscale decodes and downsizes PNG images a scanline at a time.
package pngscale

import (
	"encoding/binary"
	"errors"
	"image"
	"io"
	"math"
	"math/bits"
)

var (
	// ErrSize reports an invalid requested size or an attempt to upscale.
	ErrSize = errors.New("pngscale: invalid output dimensions")
	// ErrLimits reports that the image exceeds the decoder's size or memory limits.
	ErrLimits = errors.New("pngscale: image exceeds decoding limits")
)

// Options controls the resource limits for a single decode.
type Options struct {
	// MaxWorkingBytes bounds the scanline, accumulator, and output buffers.
	// Zero defaults to 256 MiB. The input and fixed decoder state are separate.
	MaxWorkingBytes uint64
	// MaxSourcePixels bounds the amount of image data to decompress. Zero
	// leaves the source pixel count unrestricted by this option.
	MaxSourcePixels uint64
}

// Decode returns a PNG resized to width by height by area (box) resampling.
// Each axis must be positive and at most its input dimension. The output is
// a tightly packed *image.RGBA, or *image.RGBA64 for 16-bit input. Filtering
// uses premultiplied samples and rounds once per output channel.
//
// Decode reads through IEND. It decodes the default image of animated PNGs.
// Non-interlaced images use two input scanlines and two output-width rows
// of sums. Adam7 holds the running sums for the target image across passes.
func Decode(r io.Reader, width, height int) (image.Image, error) {
	return DecodeWithOptions(r, width, height, Options{})
}

// DecodeWithOptions is Decode with resource limits for this call.
func DecodeWithOptions(r io.Reader, width, height int, opts Options) (image.Image, error) {
	if width <= 0 || height <= 0 || width > math.MaxInt32 || height > math.MaxInt32 {
		return nil, ErrSize
	}
	limit := opts.MaxWorkingBytes
	if limit == 0 {
		limit = 256 << 20
	}
	limit = min(limit, uint64(math.MaxInt))
	s := boxScaler{width: width, height: height}
	err := decodeScanlines(r, func(state scanState) error {
		return s.init(state, limit, opts.MaxSourcePixels)
	}, s.addRow)
	if err != nil {
		return nil, err
	}
	if s.interlaced && !s.direct {
		for y := range height {
			s.writeRow(y, s.vertical[y*width:][:width])
		}
	}
	return s.output, nil
}

// An axis uses the smallest integer units that express both input pixels
// and output boxes. This also eliminates overlap arithmetic for integer
// reductions, where an input pixel is exactly one unit wide.
type axis struct {
	pixel, box uint64
}

func newAxis(source, target int) axis {
	a, b := source, target
	for b != 0 {
		a, b = b, a%b
	}
	return axis{pixel: uint64(target / a), box: uint64(source / a)}
}

// RGB sums store sample*alpha, and alpha sums store alpha alone. This defers
// premultiplication's division until after the area average.
type sums struct {
	r, g, b, a uint64
}

func premultiply(r, g, b, a uint64) sums {
	return sums{r * a, g * a, b * a, a}
}

func (s *sums) add(p sums, weight uint64) {
	s.r += p.r * weight
	s.g += p.g * weight
	s.b += p.b * weight
	s.a += p.a * weight
}

func (s sums) times(weight uint64) sums {
	return sums{s.r * weight, s.g * weight, s.b * weight, s.a * weight}
}

const sumBytes = 4 * 8

type boxScaler struct {
	scanState
	width, height int
	channels      int
	x, y          axis
	maxSample     uint64
	area, divisor uint64
	powerOfTwo    bool
	areaShift     int
	direct        bool
	integralRows  bool
	output        image.Image
	pix           []byte
	paletteSums   [256]sums
	horizontal    []sums
	vertical      []sums
}

func (s *boxScaler) init(state scanState, limit, maxSourcePixels uint64) error {
	s.scanState = state
	if s.width > state.width || s.height > state.height {
		return ErrSize
	}
	if maxSourcePixels != 0 && uint64(state.width)*uint64(state.height) > maxSourcePixels {
		return ErrLimits
	}
	s.x, s.y = newAxis(state.width, s.width), newAxis(state.height, s.height)
	s.maxSample = 255
	bytesPerPixel := uint64(4)
	if state.depth == 16 {
		s.maxSample, bytesPerPixel = 65535, 8
	}
	switch state.colorType {
	case 0, 3:
		s.channels = 1
	case 2:
		s.channels = 3
	case 4:
		s.channels = 2
	case 6:
		s.channels = 4
	}
	rowBytes := uint64(scanlineSize(state.width, state.bitsPerPixel()))
	outputPixels := uint64(s.width) * uint64(s.height)
	s.direct = s.width == state.width && s.height == state.height
	s.integralRows = !state.interlaced && s.x.pixel == 1 && s.y.pixel == 1
	var verticalSize, horizontalSize uint64
	if !s.direct {
		verticalSize = uint64(s.width)
		if !s.integralRows {
			horizontalSize = uint64(s.width)
		}
		if s.interlaced {
			verticalSize = outputPixels
		}
	}
	// Also leave room for adding half the divisor when rounding.
	s.area = s.x.box * s.y.box
	if s.area > math.MaxUint64/(s.maxSample*(s.maxSample+1)) ||
		outputPixels > limit/bytesPerPixel ||
		2*rowBytes > limit-outputPixels*bytesPerPixel ||
		horizontalSize+verticalSize > (limit-outputPixels*bytesPerPixel-2*rowBytes)/sumBytes {
		return ErrLimits
	}
	s.divisor = s.area * s.maxSample
	s.powerOfTwo = s.area&(s.area-1) == 0
	s.areaShift = bits.TrailingZeros64(s.area)
	bounds := image.Rect(0, 0, s.width, s.height)
	if state.depth == 16 {
		out := image.NewRGBA64(bounds)
		s.output, s.pix = out, out.Pix
	} else {
		out := image.NewRGBA(bounds)
		s.output, s.pix = out, out.Pix
	}
	if !s.direct {
		s.horizontal = make([]sums, int(horizontalSize))
		s.vertical = make([]sums, int(verticalSize))
	}
	for i, p := range state.palette[:state.paletteLen] {
		s.paletteSums[i] = premultiply(uint64(p.R), uint64(p.G), uint64(p.B), uint64(p.A))
	}
	return nil
}

func (s *boxScaler) addRow(row []byte, x0, y, dx int) error {
	count := (s.scanState.width-1-x0)/dx + 1
	if s.colorType == 3 && s.paletteLen < 1<<s.depth {
		for i := range count {
			if int(packedSample(row, i, s.depth)) >= s.paletteLen {
				return FormatError("invalid palette index")
			}
		}
	}
	if s.direct {
		for i := range count {
			s.writePixel(y*s.width+x0+i*dx, s.sample(row, i))
		}
		return nil
	}
	if s.integralRows {
		s.addIntegralRow(row, y)
		return nil
	}
	if dx == 1 {
		s.scaleFullRow(row)
	} else {
		s.scalePassRow(row, x0, dx, count)
	}

	// An input row overlaps at most two output rows.
	top := uint64(y) * s.y.pixel
	dstY := int(top / s.y.box)
	left := s.y.box - top%s.y.box
	first := min(left, s.y.pixel)
	if s.interlaced {
		addSums(s.vertical[dstY*s.width:][:s.width], s.horizontal, first)
		if rest := s.y.pixel - first; rest != 0 {
			addSums(s.vertical[(dstY+1)*s.width:][:s.width], s.horizontal, rest)
		}
		return nil
	}
	addSums(s.vertical, s.horizontal, first)
	if first == left {
		s.writeRow(dstY, s.vertical)
		if rest := s.y.pixel - first; rest != 0 {
			for i, p := range s.horizontal {
				s.vertical[i] = p.times(rest)
			}
		} else {
			clear(s.vertical)
		}
	}
	return nil
}

func (s *boxScaler) addIntegralRow(row []byte, y int) {
	group := int(s.x.box)
	for x := range s.vertical {
		start := x * group
		s.vertical[x].add(s.sumWhole(row, start, start+group), 1)
	}
	if (y+1)%int(s.y.box) == 0 {
		s.writeRow(y/int(s.y.box), s.vertical)
		clear(s.vertical)
	}
}

func addSums(dst, src []sums, weight uint64) {
	for i, p := range src {
		dst[i].add(p, weight)
	}
}

func (s *boxScaler) scaleFullRow(row []byte) {
	if s.x.pixel == 1 {
		group := int(s.x.box)
		for x := range s.horizontal {
			start := x * group
			s.horizontal[x] = s.sumWhole(row, start, start+group)
		}
		return
	}
	var left uint64
	for x := range s.horizontal {
		right := left + s.x.box
		start, end := (left+s.x.pixel-1)/s.x.pixel, right/s.x.pixel
		total := s.sumWhole(row, int(start), int(end)).times(s.x.pixel)
		if skipped := left % s.x.pixel; skipped != 0 {
			total.add(s.sample(row, int(start)-1), s.x.pixel-skipped)
		}
		if used := right % s.x.pixel; used != 0 {
			total.add(s.sample(row, int(end)), used)
		}
		s.horizontal[x] = total
		left = right
	}
}

func (s *boxScaler) scalePassRow(row []byte, x0, dx, count int) {
	clear(s.horizontal)
	left, step := uint64(x0)*s.x.pixel, uint64(dx)*s.x.pixel
	for i := range count {
		x := int(left / s.x.box)
		first := min(s.x.pixel, s.x.box-left%s.x.box)
		p := s.sample(row, i)
		s.horizontal[x].add(p, first)
		if rest := s.x.pixel - first; rest != 0 {
			s.horizontal[x+1].add(p, rest)
		}
		left += step
	}
}

func (s *boxScaler) sumWhole(row []byte, start, end int) sums {
	var total sums
	if s.depth == 8 {
		switch {
		case s.colorType == 2 && !s.transparent:
			pixels := row[3*start : 3*end]
			for len(pixels) != 0 {
				_ = pixels[2]
				total.r += uint64(pixels[0])
				total.g += uint64(pixels[1])
				total.b += uint64(pixels[2])
				pixels = pixels[3:]
			}
			return sums{total.r * 255, total.g * 255, total.b * 255, 255 * uint64(end-start)}
		case s.colorType == 6:
			pixels := row[4*start : 4*end]
			for len(pixels) != 0 {
				a := uint64(pixels[3])
				total.r += uint64(pixels[0]) * a
				total.g += uint64(pixels[1]) * a
				total.b += uint64(pixels[2]) * a
				total.a += a
				pixels = pixels[4:]
			}
			return total
		case s.colorType == 3:
			for _, i := range row[start:end] {
				total.add(s.paletteSums[i], 1)
			}
			return total
		}
	}
	for i := start; i < end; i++ {
		total.add(s.sample(row, i), 1)
	}
	return total
}

func (s *boxScaler) sample(row []byte, i int) sums {
	if s.colorType == 3 {
		return s.paletteSums[packedSample(row, i, s.depth)]
	}
	var r, g, b uint64
	a := s.maxSample
	if s.channels <= 2 {
		if s.depth < 8 {
			r = uint64(packedSample(row, i, s.depth))
		} else {
			r = s.readSample(row, i*s.channels)
		}
		if s.colorType == 4 {
			a = s.readSample(row, i*2+1)
		} else if s.transparent && r == uint64(s.transparentR) {
			a = 0
		}
		if s.depth < 8 {
			r *= 255 / ((1 << s.depth) - 1)
		}
		g, b = r, r
	} else {
		index := i * s.channels
		r, g, b = s.readSample(row, index), s.readSample(row, index+1), s.readSample(row, index+2)
		if s.colorType == 6 {
			a = s.readSample(row, index+3)
		} else if s.transparent && r == uint64(s.transparentR) &&
			g == uint64(s.transparentG) && b == uint64(s.transparentB) {
			a = 0
		}
	}
	return premultiply(r, g, b, a)
}

func (s *boxScaler) readSample(row []byte, i int) uint64 {
	if s.depth == 16 {
		return uint64(binary.BigEndian.Uint16(row[2*i:]))
	}
	return uint64(row[i])
}

func packedSample(row []byte, i, depth int) uint8 {
	if depth == 8 {
		return row[i]
	}
	perByte := 8 / depth
	shift := (perByte - 1 - i%perByte) * depth
	return row[i/perByte] >> shift & uint8((1<<depth)-1)
}

func (s *boxScaler) writeRow(y int, row []sums) {
	for x, p := range row {
		s.writePixel(y*s.width+x, p)
	}
}

func (s *boxScaler) writePixel(i int, p sums) {
	half := s.divisor / 2
	var r, g, b, a uint64
	if s.powerOfTwo {
		// The area is a power of two for common integer reductions. Divide
		// by it first so the remaining division has a constant divisor.
		r, g, b = (p.r+half)>>s.areaShift, (p.g+half)>>s.areaShift, (p.b+half)>>s.areaShift
		a = (p.a + s.area/2) >> s.areaShift
		if s.depth == 16 {
			r, g, b = r/65535, g/65535, b/65535
		} else {
			r, g, b = r/255, g/255, b/255
		}
	} else {
		r, g, b = (p.r+half)/s.divisor, (p.g+half)/s.divisor, (p.b+half)/s.divisor
		a = (p.a + s.area/2) / s.area
	}
	if s.depth == 16 {
		pix := s.pix[i*8:][:8]
		binary.BigEndian.PutUint16(pix[0:], uint16(r))
		binary.BigEndian.PutUint16(pix[2:], uint16(g))
		binary.BigEndian.PutUint16(pix[4:], uint16(b))
		binary.BigEndian.PutUint16(pix[6:], uint16(a))
		return
	}
	pix := s.pix[i*4:][:4]
	pix[0], pix[1], pix[2], pix[3] = byte(r), byte(g), byte(b), byte(a)
}
