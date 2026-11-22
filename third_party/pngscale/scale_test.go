// spec: EV-MYGO-315 / U109
package pngscale

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/color"
	"image/png"
	"testing"
)

func TestDecodeFractionalBoxWithPaletteTransparency(t *testing.T) {
	// Three rows of packed two-bit indices into red, invisible green, blue
	// at alpha 128, and white. The low two bits in each row are padding.
	palette := []byte{255, 0, 0, 0, 255, 0, 0, 0, 255, 255, 255, 255}
	scanlines := []byte{0, 0x64, 0, 0x44, 0, 0xd8}
	compressed := deflateFixture(t, scanlines)
	fixture := pngFixture(3, 3, 2, 3, 0,
		pngChunk{"PLTE", palette},
		pngChunk{"tRNS", []byte{255, 0, 128}},
		pngChunk{"IDAT", compressed[:3]},
		pngChunk{"IDAT", nil},
		pngChunk{"IDAT", compressed[3:]})

	got, err := Decode(shortPNGReader{bytes.NewReader(fixture)}, 2, 2)
	if err != nil {
		t.Fatal(err)
	}
	rgba, ok := got.(*image.RGBA)
	if !ok {
		t.Fatalf("Decode returned %T, want *image.RGBA", got)
	}
	if rgba.Bounds() != image.Rect(0, 0, 2, 2) {
		t.Fatalf("Decode bounds = %v, want (0,0)-(2,2)", rgba.Bounds())
	}
	// Each 1.5x1.5 box contains integer weights 4, 2, 2 and 1 out of 9.
	want := [4]color.RGBA{
		{R: 28, B: 28, A: 57}, {R: 28, B: 28, A: 57},
		{R: 142, G: 113, B: 113, A: 142}, {R: 28, B: 57, A: 85},
	}
	for i, expected := range want {
		if pixel := rgba.RGBAAt(i%2, i/2); pixel != expected {
			t.Errorf("pixel %d = %v, want %v", i, pixel, expected)
		}
	}
}

func TestDecodeAdam7Preserves16BitPrecision(t *testing.T) {
	for _, tc := range []struct {
		name          string
		srcW, srcH    int
		width, height int
	}{
		{"all passes with filtered rows", 9, 7, 4, 3},
		{"empty passes at native size", 5, 3, 5, 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fixture := adam7Fixture(t, rgba64Fixture(tc.srcW, tc.srcH))
			full, err := png.Decode(bytes.NewReader(fixture))
			if err != nil {
				t.Fatalf("invalid fixture: %v", err)
			}
			source, ok := full.(*image.NRGBA64)
			if !ok {
				t.Fatalf("expected straight 16-bit alpha, got %T", full)
			}
			got, err := Decode(bytes.NewReader(fixture), tc.width, tc.height)
			if err != nil {
				t.Fatal(err)
			}
			rgba, ok := got.(*image.RGBA64)
			if !ok {
				t.Fatalf("Decode returned %T, want *image.RGBA64", got)
			}
			if want := image.Rect(0, 0, tc.width, tc.height); rgba.Bounds() != want {
				t.Fatalf("Decode bounds = %v, want %v", rgba.Bounds(), want)
			}
			for y := range tc.height {
				for x := range tc.width {
					want := referenceBox16(source, x, y, tc.width, tc.height)
					if pixel := rgba.RGBA64At(x, y); pixel != want {
						t.Errorf("pixel (%d,%d) = %v, want %v", x, y, pixel, want)
					}
				}
			}
		})
	}
}

func TestDecodeRejectsIncompleteImageData(t *testing.T) {
	raw := []byte{0, 40, 0, 100}
	idat := deflateFixture(t, raw)
	makeFixture := func(compressed []byte) []byte {
		return pngFixture(1, 2, 8, 0, 0, pngChunk{"IDAT", compressed})
	}
	valid := makeFixture(idat)
	badCRC := bytes.Clone(valid)
	badCRC[len(badCRC)-13] ^= 1 // Last byte of the IDAT CRC.
	badAdler := bytes.Clone(idat)
	badAdler[len(badAdler)-1] ^= 1

	for _, tc := range []struct {
		name    string
		fixture []byte
	}{
		{"missing IEND checksum", valid[:len(valid)-3]},
		{"wrong IDAT CRC", badCRC},
		{"wrong zlib Adler-32", makeFixture(badAdler)},
		{"missing scanline", makeFixture(deflateFixture(t, raw[:2]))},
		{"extra inflated bytes", makeFixture(deflateFixture(t, append(bytes.Clone(raw), 0)))},
		{"invalid row filter", makeFixture(deflateFixture(t, []byte{5, 40, 0, 100}))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got, err := Decode(bytes.NewReader(tc.fixture), 1, 1); got != nil || err == nil {
				t.Fatalf("Decode = (%T, %v), want nil image and an error", got, err)
			}
		})
	}
}

func TestDecodeTallPNGUnderSmallWorkingLimit(t *testing.T) {
	const width, height = 129, 513
	const dstW, dstH = 17, 5
	src := image.NewGray(image.Rect(0, 0, width, height))
	for i := range src.Pix {
		src.Pix[i] = 131
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, src); err != nil {
		t.Fatal(err)
	}

	// The budget fits a few scanlines plus the target, but even an 8-bit
	// buffer with all source rows at the target width needs over 8 KiB.
	const budget = 2 << 10
	got, err := DecodeWithOptions(bytes.NewReader(encoded.Bytes()), dstW, dstH, Options{MaxWorkingBytes: budget})
	if err != nil {
		t.Fatal(err)
	}
	scaled, ok := got.(*image.RGBA)
	if !ok {
		t.Fatalf("DecodeWithOptions returned %T, want *image.RGBA", got)
	}
	if p := scaled.RGBAAt(dstW-1, dstH-1); p != (color.RGBA{R: 131, G: 131, B: 131, A: 255}) {
		t.Fatalf("last pixel = %v", p)
	}

	for _, opts := range []Options{
		{MaxWorkingBytes: 128},
		{MaxWorkingBytes: budget, MaxSourcePixels: width*height - 1},
	} {
		got, err := DecodeWithOptions(bytes.NewReader(encoded.Bytes()), dstW, dstH, opts)
		if got != nil || !errors.Is(err, ErrLimits) {
			t.Fatalf("DecodeWithOptions(%+v) = (%T, %v), want ErrLimits and nil image", opts, got, err)
		}
	}
}

// referenceBox16 integrates each source pixel's area directly in the
// original raster. The overlap weights are exact integers in a grid scaled
// by the target width and height.
func referenceBox16(source *image.NRGBA64, x, y, width, height int) color.RGBA64 {
	srcW, srcH := source.Bounds().Dx(), source.Bounds().Dy()
	left, right := x*srcW, (x+1)*srcW
	top, bottom := y*srcH, (y+1)*srcH
	var red, green, blue, alpha uint64
	for sy := top / height; sy*height < bottom; sy++ {
		h := min(bottom, (sy+1)*height) - max(top, sy*height)
		for sx := left / width; sx*width < right; sx++ {
			w := min(right, (sx+1)*width) - max(left, sx*width)
			p := source.NRGBA64At(sx, sy)
			a := uint64(p.A) * uint64(w*h)
			red += uint64(p.R) * a
			green += uint64(p.G) * a
			blue += uint64(p.B) * a
			alpha += a
		}
	}
	area := uint64(srcW * srcH)
	rgbDivisor := area * 65535
	return color.RGBA64{
		R: uint16((red + rgbDivisor/2) / rgbDivisor),
		G: uint16((green + rgbDivisor/2) / rgbDivisor),
		B: uint16((blue + rgbDivisor/2) / rgbDivisor),
		A: uint16((alpha + area/2) / area),
	}
}

type shortPNGReader struct {
	source *bytes.Reader
}

func (r shortPNGReader) Read(p []byte) (int, error) {
	return r.source.Read(p[:min(3, len(p))])
}

type pngChunk struct {
	name string
	data []byte
}

func pngFixture(width, height int, depth, colorType, interlace byte, chunks ...pngChunk) []byte {
	var header [13]byte
	binary.BigEndian.PutUint32(header[:4], uint32(width))
	binary.BigEndian.PutUint32(header[4:8], uint32(height))
	header[8], header[9], header[12] = depth, colorType, interlace

	out := bytes.NewBufferString("\x89PNG\r\n\x1a\n")
	writePNGChunk(out, pngChunk{"IHDR", header[:]})
	for _, chunk := range chunks {
		writePNGChunk(out, chunk)
	}
	writePNGChunk(out, pngChunk{"IEND", nil})
	return out.Bytes()
}

func writePNGChunk(out *bytes.Buffer, chunk pngChunk) {
	var field [4]byte
	binary.BigEndian.PutUint32(field[:], uint32(len(chunk.data)))
	out.Write(field[:])
	start := out.Len()
	out.WriteString(chunk.name)
	out.Write(chunk.data)
	binary.BigEndian.PutUint32(field[:], crc32.ChecksumIEEE(out.Bytes()[start:]))
	out.Write(field[:])
}

func deflateFixture(t testing.TB, raw []byte) []byte {
	t.Helper()
	var compressed bytes.Buffer
	z := zlib.NewWriter(&compressed)
	if _, err := z.Write(raw); err != nil {
		t.Fatal(err)
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	return compressed.Bytes()
}

func rgba64Fixture(width, height int) *image.NRGBA64 {
	source := image.NewNRGBA64(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
			alpha := uint16(10943 + x*201 + y*107)
			if (x+y)%4 == 0 {
				alpha = 0
			}
			source.SetNRGBA64(x, y, color.NRGBA64{
				R: uint16(40001 + x*379 + y*613),
				G: uint16(12345 + x*181 + y*83),
				B: uint16(60033 - x*317 - y*503),
				A: alpha,
			})
		}
	}
	return source
}

func adam7Fixture(t testing.TB, source *image.NRGBA64) []byte {
	t.Helper()
	// The entries give the pass number of each pixel in an 8x8 Adam7 tile.
	tile := [8][8]byte{
		{1, 6, 4, 6, 2, 6, 4, 6},
		{7, 7, 7, 7, 7, 7, 7, 7},
		{5, 6, 5, 6, 5, 6, 5, 6},
		{7, 7, 7, 7, 7, 7, 7, 7},
		{3, 6, 4, 6, 3, 6, 4, 6},
		{7, 7, 7, 7, 7, 7, 7, 7},
		{5, 6, 5, 6, 5, 6, 5, 6},
		{7, 7, 7, 7, 7, 7, 7, 7},
	}
	width, height := source.Bounds().Dx(), source.Bounds().Dy()
	var compressed bytes.Buffer
	z, err := zlib.NewWriterLevel(&compressed, zlib.BestSpeed)
	if err != nil {
		t.Fatal(err)
	}
	prev := make([]byte, 8*width)
	raw := make([]byte, 0, 8*width)
	filtered := make([]byte, 1+8*width)
	for pass := byte(1); pass <= 7; pass++ {
		clear(prev)
		for y := range height {
			raw = raw[:0]
			for x := range width {
				if tile[y%8][x%8] == pass {
					i := source.PixOffset(x, y)
					raw = append(raw, source.Pix[i:i+8]...)
				}
			}
			if len(raw) == 0 {
				continue
			}
			// A mix of Sub and Up exercises both scanline neighbors.
			filter := byte(1 + y%2)
			filtered[0] = filter
			for i, v := range raw {
				if filter == 2 {
					v -= prev[i]
				} else if i >= 8 {
					v -= raw[i-8]
				}
				filtered[1+i] = v
			}
			if _, err := z.Write(filtered[:len(raw)+1]); err != nil {
				t.Fatal(err)
			}
			copy(prev, raw)
		}
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	const chunkSize = 32 << 10
	var chunks []pngChunk
	data := compressed.Bytes()
	for len(data) > 0 {
		n := min(len(data), chunkSize)
		chunks = append(chunks, pngChunk{"IDAT", data[:n]})
		data = data[n:]
	}
	return pngFixture(width, height, 16, 6, 1, chunks...)
}
