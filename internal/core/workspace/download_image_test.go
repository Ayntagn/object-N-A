// spec: FEAT-NOTE-DROP / EV-MYGO-282
package workspace

import (
	"bytes"
	"context"
	"image"
	"image/jpeg"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
)

func TestDownloadImageSavesOriginalBytes(t *testing.T) {
	var imageBytes bytes.Buffer
	if err := jpeg.Encode(&imageBytes, image.NewRGBA(image.Rect(0, 0, 2, 3)), nil); err != nil {
		t.Fatal(err)
	}
	fixture := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/drop.png" {
			http.Redirect(w, r, "http://cdn.example/asset", http.StatusTemporaryRedirect)
			return
		}
		if r.Header.Get("Referer") != "" {
			t.Error("redirect sent the original image URL")
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write(imageBytes.Bytes())
	}))
	defer fixture.Close()
	client := newImageDownloadClient()
	defer client.CloseIdleConnections()
	client.Transport.(*http.Transport).DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, fixture.Listener.Addr().String())
	}
	const address = "http://pictures.example/drop.png?token=fixture"
	got, err := downloadImage(context.Background(), client, address)
	if err != nil || !bytes.Equal(got, imageBytes.Bytes()) {
		t.Fatalf("downloaded %d bytes, error: %v", len(got), err)
	}

	dir := t.TempDir()
	v, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = v.Close() })
	if _, err := v.CreateTextDocument("notes/note.md", "original"); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"images/drop.jpg", "images/drop-1.jpg"} {
		destination, err := v.ImportDownloadedImage("notes/note.md", address, got)
		if err != nil || destination != want {
			t.Fatalf("destination = %q, error = %v, want %q", destination, err, want)
		}
		relative, ok := ResolveImagePath("notes/note.md", destination)
		data, mime, err := v.ReadPreviewImage(relative, maxImageBytes)
		if !ok || err != nil || mime != "image/jpeg" || !bytes.Equal(data, got) {
			t.Fatalf("preview attachment = %q, %s, error = %v", relative, mime, err)
		}
	}
	note, err := os.ReadFile(filepath.Join(dir, "notes", "note.md"))
	if err != nil || string(note) != "original" {
		t.Fatalf("note = %q, error = %v", note, err)
	}
}

func TestDownloadImageRejectsNonPublicRedirect(t *testing.T) {
	var requests atomic.Int32
	fixture := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		http.Redirect(w, r, "http://127.0.0.1/private.png", http.StatusFound)
	}))
	defer fixture.Close()
	client := newImageDownloadClient()
	defer client.CloseIdleConnections()
	client.Transport.(*http.Transport).DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, fixture.Listener.Addr().String())
	}
	if _, err := downloadImage(context.Background(), client, "http://pictures.example/public.png"); err == nil || requests.Load() != 1 {
		t.Fatalf("private redirect returned error %v after %d requests", err, requests.Load())
	}
}

func TestDownloadImageHonorsSystemFakeIPForQualifiedHosts(t *testing.T) {
	// The macOS drop replay resolved this public image host through the
	// configured Fake-IP route.
	ip := netip.MustParseAddr("198.18.0.116")
	if !imageDialAddressAllowed("www.w3.org", ip) {
		t.Fatal("qualified hostname cannot use its resolved Fake-IP address")
	}
	for _, host := range []string{ip.String(), "printer", "printer.local", "router.home.arpa"} {
		if imageDialAddressAllowed(host, ip) {
			t.Errorf("accepted a Fake-IP route for %q", host)
		}
	}
	if imageDialAddressAllowed("www.w3.org", netip.MustParseAddr("192.168.0.1")) {
		t.Error("accepted a real private address for a qualified hostname")
	}
	if _, err := DownloadImage(context.Background(), "http://198.18.0.116/image.png"); err != errPrivateImageURL {
		t.Fatalf("literal Fake-IP URL returned %v", err)
	}
}
