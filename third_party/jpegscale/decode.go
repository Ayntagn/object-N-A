// Package jpegscale decodes JPEG images with optional reduced-size IDCT.
package jpegscale

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"math"

	"object-na.local/jpegscale/internal/jpegn"
)

const (
	defaultMaxPixels = 32 << 20
	maxSourcePixels  = min(1<<28, math.MaxInt>>6)
)

var (
	// ErrScale indicates a denominator other than 0, 1, 2, 4, or 8.
	ErrScale = errors.New("jpegscale: invalid scale denominator")
	// ErrImageTooLarge indicates that the source dimensions exceed the pixel limit.
	ErrImageTooLarge = errors.New("jpegscale: source pixel limit exceeded")
	// ErrUnsupported indicates that a fully validated JPEG could not be
	// represented compatibly by the reduced IDCT decoder.
	ErrUnsupported = errors.New("jpegscale: reduced IDCT unsupported")
)

// Options controls a single decode.
type Options struct {
	// ScaleDenom selects the IDCT scale. Zero or 1 returns the full-size
	// standard-library image. Values 2, 4, and 8 round each output dimension up.
	ScaleDenom int
	// MaxPixels limits the source width times height before any pixel allocation.
	// Zero allows 32 Mi pixels. The decoder also observes its platform limit.
	MaxPixels uint64
}

// Decode validates the first JPEG image in data through its end marker and
// decodes its complete scan data with image/jpeg. If requested, it then
// reconstructs the smaller image using reduced IDCT. EXIF orientation stays as
// stored in the file. Keep data unchanged for the duration of the call.
func Decode(data []byte, opts Options) (image.Image, error) {
	switch opts.ScaleDenom {
	case 0, 1, 2, 4, 8:
	default:
		return nil, fmt.Errorf("%w: %d", ErrScale, opts.ScaleDenom)
	}
	limit := opts.MaxPixels
	if limit == 0 {
		limit = defaultMaxPixels
	}
	width, height, end, err := inspectFrame(data, min(limit, maxSourcePixels))
	if err != nil {
		return nil, err
	}
	data = data[:end]

	full, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	if opts.ScaleDenom < 2 {
		return full, nil
	}
	model := full.ColorModel()

	scaled, err := jpegn.Decode(data, opts.ScaleDenom)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUnsupported, err)
	}
	denom := opts.ScaleDenom
	want := image.Rect(0, 0, (width+denom-1)/denom, (height+denom-1)/denom)
	if scaled == nil || scaled.Bounds() != want {
		return nil, fmt.Errorf("%w: inconsistent image bounds", ErrUnsupported)
	}
	if scaled.ColorModel() != model {
		return nil, fmt.Errorf("%w: inconsistent color model", ErrUnsupported)
	}
	return scaled, nil
}
