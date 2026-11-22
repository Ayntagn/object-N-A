// Package jpegn embeds the decoder of github.com/gen2brain/jpegn v0.7.1.
package jpegn

import (
	"errors"
	"image"
	"sync"
)

var (
	ErrNoJPEG             = errors.New("not a JPEG file")
	ErrUnsupported        = errors.New("unsupported format")
	ErrOutOfMemory        = errors.New("out of memory")
	ErrInternal           = errors.New("internal error")
	ErrSyntax             = errors.New("syntax error")
	ErrMissingHuffmanCode = errors.New("missing Huffman code")
	ErrInvalidSegment     = errors.New("invalid marker segment")
)

type UpsampleMethod int

const (
	NearestNeighbor UpsampleMethod = iota
	CatmullRom
)

var decoderPool = sync.Pool{
	New: func() any { return newDecoder() },
}

// Decode runs the reduced inverse transform on data already validated by the
// caller. It reads the caller's bytes directly and decodes in storage orientation.
func Decode(data []byte, denominator int) (image.Image, error) {
	d := decoderPool.Get().(*decoder)
	d.reset()
	defer func() {
		d.reset()
		decoderPool.Put(d)
	}()
	d.scaleDenom = denominator
	return d.decode(data, false)
}
