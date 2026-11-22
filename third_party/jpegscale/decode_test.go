// spec: EV-MYGO-317 / U109

package jpegscale_test

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"os"
	"testing"

	"object-na.local/jpegscale"
)

func TestDecodeAtRequestedScale(t *testing.T) {
	for _, sample := range []string{"baseline-restarts", "progressive"} {
		t.Run(sample, func(t *testing.T) {
			data, err := os.ReadFile("testdata/" + sample + ".jpg")
			if err != nil {
				t.Fatal(err)
			}
			for _, scale := range []struct{ denom, width, height int }{
				{0, 37, 29}, {1, 37, 29}, {2, 19, 15}, {4, 10, 8}, {8, 5, 4},
			} {
				img, err := jpegscale.Decode(data, jpegscale.Options{ScaleDenom: scale.denom})
				if err != nil {
					t.Fatalf("scale 1/%d: %v", scale.denom, err)
				}
				want := image.Rect(0, 0, scale.width, scale.height)
				if img.Bounds() != want {
					t.Fatalf("scale 1/%d: bounds = %v, want %v", scale.denom, img.Bounds(), want)
				}
				// The fixture is a gradient along both axes. Dimensions alone
				// would also accept an unfilled output buffer.
				r0, g0, _, _ := img.At(0, 0).RGBA()
				r1, g1, _, _ := img.At(scale.width-1, scale.height-1).RGBA()
				if r1 <= r0+20000 || g1 <= g0+20000 {
					t.Fatalf("scale 1/%d: gradient lost (%d,%d) to (%d,%d)", scale.denom, r0, g0, r1, g1)
				}
			}
		})
	}
}

func TestDecodeRejectsTruncatedJPEG(t *testing.T) {
	baseline := encodeJPEG(t, 37, 29)
	sequential, err := os.ReadFile("testdata/sequential-scans.jpg")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := jpegscale.Decode(sequential, jpegscale.Options{ScaleDenom: 4}); err != nil {
		t.Fatalf("complete sequential scans: %v", err)
	}
	missingComponent := endAfterFirstScan(t, sequential)
	if _, err := jpeg.Decode(bytes.NewReader(missingComponent)); err != nil {
		t.Fatalf("missing-component fixture should exercise stricter validation than image/jpeg: %v", err)
	}
	for _, sample := range []struct {
		name string
		data []byte
	}{
		{"missing EOI", baseline[:len(baseline)-2]},
		{"EOI before end of entropy data", withEOI(baseline[:len(baseline)-32])},
		{"EOI without all frame components", missingComponent},
	} {
		t.Run(sample.name, func(t *testing.T) {
			for _, denom := range []int{1, 4} {
				img, err := jpegscale.Decode(sample.data, jpegscale.Options{ScaleDenom: denom})
				if err == nil || img != nil || errors.Is(err, jpegscale.ErrUnsupported) {
					t.Fatalf("scale 1/%d: got image %t and %v", denom, img != nil, err)
				}
			}
		})
	}
}

func TestDecodePermitsCompleteProgressiveDCScan(t *testing.T) {
	data, err := os.ReadFile("testdata/progressive.jpg")
	if err != nil {
		t.Fatal(err)
	}
	img, err := jpegscale.Decode(endAfterFirstScan(t, data), jpegscale.Options{ScaleDenom: 4})
	if err != nil {
		t.Fatal(err)
	}
	if want := image.Rect(0, 0, 10, 8); img.Bounds() != want {
		t.Fatalf("DC-only progressive bounds = %v, want %v", img.Bounds(), want)
	}
}

func TestDecodeLimits(t *testing.T) {
	data := encodeJPEG(t, 37, 29)
	img, err := jpegscale.Decode(data, jpegscale.Options{ScaleDenom: 4, MaxPixels: 1024})
	if img != nil || !errors.Is(err, jpegscale.ErrImageTooLarge) {
		t.Fatalf("pixel limit: got image %t and %v", img != nil, err)
	}
	img, err = jpegscale.Decode(data, jpegscale.Options{ScaleDenom: 3})
	if img != nil || !errors.Is(err, jpegscale.ErrScale) {
		t.Fatalf("invalid denominator: got image %t and %v", img != nil, err)
	}
}

func TestReducedDecodeReportsRestartRecovery(t *testing.T) {
	data, err := os.ReadFile("testdata/baseline-restarts.jpg")
	if err != nil {
		t.Fatal(err)
	}
	index := bytes.Index(data, []byte{0xff, 0xd0})
	if index < 0 {
		t.Fatal("fixture has no restart marker")
	}
	// image/jpeg tolerates stray data while finding the expected restart.
	// The scaled backend must signal fallback instead of synthesizing pixels.
	withGap := make([]byte, len(data)+1)
	copy(withGap, data[:index])
	withGap[index] = 0x7e
	copy(withGap[index+1:], data[index:])
	if _, err := jpegscale.Decode(withGap, jpegscale.Options{}); err != nil {
		t.Fatalf("full standard decode: %v", err)
	}
	img, err := jpegscale.Decode(withGap, jpegscale.Options{ScaleDenom: 4})
	if img != nil || !errors.Is(err, jpegscale.ErrUnsupported) {
		t.Fatalf("reduced decode: got image %t and %v", img != nil, err)
	}
}

func endAfterFirstScan(t *testing.T, data []byte) []byte {
	t.Helper()
	// The two tiny cjpeg fixtures contain no APP or table data with this
	// byte sequence. For progressive, the earliest subsequent marker may
	// be a DHT immediately before the second scan.
	start := bytes.Index(data, []byte{0xff, 0xda})
	if start < 0 {
		t.Fatal("fixture has no SOS")
	}
	next := bytes.Index(data[start+2:], []byte{0xff, 0xda})
	if next < 0 {
		t.Fatal("fixture has no second SOS")
	}
	return withEOI(data[:start+2+next])
}

func withEOI(data []byte) []byte {
	return append(bytes.Clone(data), 0xff, 0xd9)
}

func encodeJPEG(t testing.TB, width, height int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
			img.SetRGBA(x, y, color.RGBA{
				R: uint8(222*x/(width-1) + 11*(x/64&1)),
				G: uint8(224*y/(height-1) + 13*(y/64&1)),
				B: uint8((x*4 + y*3) % 256),
				A: 255,
			})
		}
	}
	var encoded bytes.Buffer
	if err := jpeg.Encode(&encoded, img, &jpeg.Options{Quality: 85}); err != nil {
		t.Fatal(err)
	}
	return encoded.Bytes()
}

func BenchmarkDecodePreview(b *testing.B) {
	const size = 4096
	data := encodeJPEG(b, size, size)
	b.Run("StandardDecode", func(b *testing.B) {
		for b.Loop() {
			if _, err := jpeg.Decode(bytes.NewReader(data)); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("StandardDecodeThenRGBAThenBox", func(b *testing.B) {
		for b.Loop() {
			img, err := jpeg.Decode(bytes.NewReader(data))
			if err != nil {
				b.Fatal(err)
			}
			rgba := asRGBA(img)
			preview := image.NewRGBA(image.Rect(0, 0, size/4, size/4))
			for y := range size / 4 {
				for x := range size / 4 {
					var r, g, b int
					for dy := range 4 {
						for dx := range 4 {
							c := rgba.RGBAAt(4*x+dx, 4*y+dy)
							r += int(c.R)
							g += int(c.G)
							b += int(c.B)
						}
					}
					preview.SetRGBA(x, y, color.RGBA{uint8(r / 16), uint8(g / 16), uint8(b / 16), 255})
				}
			}
		}
	})
	b.Run("StandardDecodeThenYCbCrBox", func(b *testing.B) {
		for b.Loop() {
			img, err := jpeg.Decode(bytes.NewReader(data))
			if err != nil {
				b.Fatal(err)
			}
			ycbcr, ok := img.(*image.YCbCr)
			if !ok || ycbcr.SubsampleRatio != image.YCbCrSubsampleRatio420 {
				b.Fatalf("expected a 4:2:0 source, got %T", img)
			}
			_ = ycbcrBox4(ycbcr)
		}
	})
	b.Run("StrictReducedIDCTThenRGBA", func(b *testing.B) {
		for b.Loop() {
			img, err := jpegscale.Decode(data, jpegscale.Options{ScaleDenom: 4})
			if err != nil {
				b.Fatal(err)
			}
			_ = asRGBA(img)
		}
	})
}

// ycbcrBox4 is the direct 4:2:0 reference path for a source divisible by four.
// It averages samples in YCbCr before converting the destination to RGBA.
func ycbcrBox4(src *image.YCbCr) *image.RGBA {
	width, height := src.Rect.Dx()/4, src.Rect.Dy()/4
	dst := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		lumaRow := 4 * y * src.YStride
		chromaRow := 2 * y * src.CStride
		for x := range width {
			lumaSum := 0
			for dy := range 4 {
				p := src.Y[lumaRow+dy*src.YStride+4*x:][:4]
				lumaSum += int(p[0]) + int(p[1]) + int(p[2]) + int(p[3])
			}
			first := chromaRow + 2*x
			second := first + src.CStride
			cb := int(src.Cb[first]) + int(src.Cb[first+1]) + int(src.Cb[second]) + int(src.Cb[second+1])
			cr := int(src.Cr[first]) + int(src.Cr[first+1]) + int(src.Cr[second]) + int(src.Cr[second+1])
			r, g, b := color.YCbCrToRGB(uint8((lumaSum+8)/16), uint8((cb+2)/4), uint8((cr+2)/4))
			p := dst.Pix[y*dst.Stride+4*x:][:4]
			p[0], p[1], p[2], p[3] = r, g, b, 255
		}
	}
	return dst
}

func asRGBA(src image.Image) *image.RGBA {
	dst := image.NewRGBA(src.Bounds())
	draw.Draw(dst, dst.Bounds(), src, src.Bounds().Min, draw.Src)
	return dst
}
