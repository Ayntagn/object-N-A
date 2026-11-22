// spec: EV-MYGO-317 / U109

package jpegn

import (
	"bytes"
	"errors"
	"image"
	"testing"
)

// Keep image/jpeg out of this test binary to observe changes to the registry
// from the bundled decoder alone.
func TestImportDoesNotRegisterJPEG(t *testing.T) {
	data := []byte{0xff, 0xd8, 0xff, 0xd9}
	if _, _, err := image.Decode(bytes.NewReader(data)); !errors.Is(err, image.ErrFormat) {
		t.Fatalf("image.Decode recognized JPEG with only the backend imported: %v", err)
	}
}
