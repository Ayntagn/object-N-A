package ui

import (
	"bytes"
	"errors"
	"image"
	"image/png"

	"object-na.local/jpegscale"
	"object-na.local/pngscale"
)

// ReadPreviewImage limits the dimensions before MyGo requests a preview.
// Returning nil without an error lets MyGo decode the original normally.
func decodeVisualPNGPreview(data []byte, denominator int) (image.Image, error) {
	if denominator < 2 {
		return nil, nil
	}
	config, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	width := (config.Width-1)/denominator + 1
	height := (config.Height-1)/denominator + 1
	img, err := pngscale.Decode(bytes.NewReader(data), width, height)
	if errors.Is(err, pngscale.ErrLimits) {
		return nil, nil
	}
	return img, err
}

func decodeVisualJPEGPreview(data []byte, denominator int) (image.Image, error) {
	img, err := jpegscale.Decode(data, jpegscale.Options{ScaleDenom: denominator})
	if errors.Is(err, jpegscale.ErrUnsupported) {
		return nil, nil
	}
	return img, err
}
