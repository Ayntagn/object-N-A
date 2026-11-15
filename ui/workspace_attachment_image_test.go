// spec: user-requested single-pane attachment reading and reversible zoom.
package ui

import (
	"image"
	"testing"

	mygoui "github.com/egoist/mygo/ui"
)

func TestImageAttachmentFitsZoomsAndResizes(t *testing.T) {
	viewer := workspaceImageWindow{
		absolute: "Reader.png",
		bitmap:   mygoui.NewBitmap(image.NewRGBA(image.Rect(0, 0, 800, 400))),
	}
	test := mygoui.NewTester(viewer.draw, 600, 400)
	size := func(wantWidth, wantHeight float32) {
		t.Helper()
		picture, ok := test.AccessByRole(mygoui.RoleImage, "Reader.png")
		if !ok || picture.Bounds.W != wantWidth || picture.Bounds.H != wantHeight {
			t.Fatalf("image bounds = %+v, want %gx%g", picture.Bounds, wantWidth, wantHeight)
		}
	}
	click := func(label string) {
		t.Helper()
		if err := test.Click(label); err != nil {
			t.Fatal(err)
		}
	}
	size(568, 284)
	click("Zoom In")
	size(768, 384)
	click("Actual Size")
	size(800, 400)
	click("Fit Image to Window")
	test.SetSize(420, 400)
	size(388, 194)
}
