// spec: EV-MYGO-315 / U109
package pngscale

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/png"
	"testing"
)

func BenchmarkDecodeBox(b *testing.B) {
	b.Run("RGB8_4096_to_1024", func(b *testing.B) {
		const side = 4096
		src := image.NewRGBA(image.Rect(0, 0, side, side))
		for y := range side {
			row := src.Pix[y*src.Stride : (y+1)*src.Stride]
			for x := range side {
				i := x * 4
				row[i] = byte(x/16 + y/7)
				row[i+1] = byte((x + y) / 13)
				row[i+2] = byte(x/5 + (y / 3) ^ (x / 9))
				row[i+3] = 255
			}
		}
		var fixture bytes.Buffer
		encoder := png.Encoder{CompressionLevel: png.BestSpeed}
		if err := encoder.Encode(&fixture, src); err != nil {
			b.Fatal(err)
		}
		comparePNGPaths(b, fixture.Bytes(), side/4, func(full image.Image) []byte {
			return box4RGB8(full.(*image.RGBA)).Pix
		})
	})

	b.Run("Adam7_RGBA16_2048_to_512", func(b *testing.B) {
		const side = 2048
		fixture := adam7Fixture(b, rgba64Fixture(side, side))
		comparePNGPaths(b, fixture, side/4, func(full image.Image) []byte {
			return box4RGBA16(full.(*image.NRGBA64)).Pix
		})
	})
}

func comparePNGPaths(b *testing.B, fixture []byte, side int, scaleFull func(image.Image) []byte) {
	b.Helper()
	streaming := func() ([]byte, error) {
		scaled, err := Decode(bytes.NewReader(fixture), side, side)
		if err != nil {
			return nil, err
		}
		switch result := scaled.(type) {
		case *image.RGBA:
			return result.Pix, nil
		case *image.RGBA64:
			return result.Pix, nil
		default:
			b.Fatalf("unexpected streaming output %T", scaled)
			return nil, nil
		}
	}
	stdlibBox := func() ([]byte, error) {
		full, err := png.Decode(bytes.NewReader(fixture))
		if err != nil {
			return nil, err
		}
		return scaleFull(full), nil
	}

	got, err := streaming()
	if err != nil {
		b.Fatal(err)
	}
	want, err := stdlibBox()
	if err != nil {
		b.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		b.Fatal("streaming and full-frame box pixels differ")
	}

	for _, tc := range []struct {
		name   string
		decode func() ([]byte, error)
	}{
		{"streaming", streaming},
		{"stdlib_box", stdlibBox},
	} {
		b.Run(tc.name, func(b *testing.B) {
			b.SetBytes(int64(len(fixture)))
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if _, err := tc.decode(); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func box4RGB8(src *image.RGBA) *image.RGBA {
	width, height := src.Bounds().Dx()/4, src.Bounds().Dy()/4
	dst := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
			var r, g, b uint32
			for dy := range 4 {
				offset := (4*y+dy)*src.Stride + 16*x
				row := src.Pix[offset : offset+16]
				for i := 0; i < len(row); i += 4 {
					r += uint32(row[i])
					g += uint32(row[i+1])
					b += uint32(row[i+2])
				}
			}
			i := y*dst.Stride + 4*x
			dst.Pix[i] = byte((r + 8) / 16)
			dst.Pix[i+1] = byte((g + 8) / 16)
			dst.Pix[i+2] = byte((b + 8) / 16)
			dst.Pix[i+3] = 255
		}
	}
	return dst
}

func box4RGBA16(src *image.NRGBA64) *image.RGBA64 {
	width, height := src.Bounds().Dx()/4, src.Bounds().Dy()/4
	dst := image.NewRGBA64(image.Rect(0, 0, width, height))
	const colorDivisor = 16 * 65535
	for y := range height {
		for x := range width {
			var r, g, b, a uint64
			for dy := range 4 {
				offset := (4*y+dy)*src.Stride + 32*x
				row := src.Pix[offset : offset+32]
				for i := 0; i < len(row); i += 8 {
					alpha := uint64(binary.BigEndian.Uint16(row[i+6:]))
					r += uint64(binary.BigEndian.Uint16(row[i:])) * alpha
					g += uint64(binary.BigEndian.Uint16(row[i+2:])) * alpha
					b += uint64(binary.BigEndian.Uint16(row[i+4:])) * alpha
					a += alpha
				}
			}
			i := y*dst.Stride + 8*x
			binary.BigEndian.PutUint16(dst.Pix[i:], uint16((r+colorDivisor/2)/colorDivisor))
			binary.BigEndian.PutUint16(dst.Pix[i+2:], uint16((g+colorDivisor/2)/colorDivisor))
			binary.BigEndian.PutUint16(dst.Pix[i+4:], uint16((b+colorDivisor/2)/colorDivisor))
			binary.BigEndian.PutUint16(dst.Pix[i+6:], uint16((a+8)/16))
		}
	}
	return dst
}
