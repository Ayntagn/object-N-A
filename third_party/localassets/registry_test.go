package localassets

// spec: EV-MYGO-314 / EV-UI-052 / EV-MYGO-326

import (
	"bytes"
	"encoding/binary"
	"strings"
	"testing"
)

func TestRegistryProtectsImmutableOwnerAssets(t *testing.T) {
	r := New[string](Limits{MaxBytes: 12, MaxRoutes: 3, MaxOwnerBytes: 8, MaxOwnerRoutes: 2})
	source := []byte("fixture")
	grant, ok := r.AddBytes("preview", "application/pdf", source)
	if !ok {
		t.Fatal("grant was refused")
	}
	source[0] = 'X'
	if _, found := r.Read("another page", grant); found {
		t.Fatal("another page read the grant")
	}
	asset, found := r.Read("preview", grant)
	var body bytes.Buffer
	_, err := asset.WriteTo(&body)
	if !found || err != nil || asset.ContentType() != "application/pdf" || body.String() != "fixture" {
		t.Fatalf("immutable grant = %q, %v, %v", body.String(), found, err)
	}
	if _, ok := r.AddText("preview", "text/plain", "二"); ok {
		t.Fatal("owner's byte limit was bypassed")
	}
	if _, ok := r.AddText("another page", "text/plain", "cross"); !ok {
		t.Fatal("other page should have its own allowance")
	}
	r.Revoke("another page", grant)
	if _, found := r.Read("preview", grant); !found {
		t.Fatal("another page revoked the grant")
	}
	r.RevokeOwner("preview")
	if _, found := r.Read("preview", grant); found {
		t.Fatal("revoked grant remained readable")
	}
	if _, ok := r.AddText("preview", "text/plain", "released"); ok {
		t.Fatal("total byte limit was bypassed")
	}
	if _, ok := r.AddText("preview", "text/plain", "release"); !ok {
		t.Fatal("revoking grants did not return their allowances")
	}
}

func TestRegistryReceivesOnlyOneCompleteUpload(t *testing.T) {
	r := New[int](Limits{MaxBytes: 20, MaxRoutes: 3})
	const owner = 1
	token, ok := r.ReserveUpload(owner, 10)
	if !ok {
		t.Fatal("reserve upload")
	}
	upload := func(body string, size int) *bytes.Reader {
		frame := binary.BigEndian.AppendUint32(nil, uint32(size))
		return bytes.NewReader(append(frame, body...))
	}
	if err := r.ReceiveUpload(2, token, upload("data", 4)); err == nil {
		t.Fatal("another page submitted this result")
	}
	if err := r.ReceiveUpload(owner, token, upload("data", 4)); err != nil {
		t.Fatal(err)
	}
	if err := r.ReceiveUpload(owner, token, upload("again", 5)); err == nil {
		t.Fatal("a second request replaced the result")
	}
	data, ok := r.TakeUpload(owner, token)
	if !ok || string(data) != "data" {
		t.Fatalf("result = %q, %v", data, ok)
	}
	if _, found := r.TakeUpload(owner, token); found {
		t.Fatal("a second reader found the result")
	}
	malformed, _ := r.ReserveUpload(owner, 10)
	if err := r.ReceiveUpload(owner, malformed, upload(strings.Repeat("a", 11), 11)); err == nil {
		t.Fatal("oversized upload was accepted")
	}
	if _, found := r.TakeUpload(owner, malformed); found {
		t.Fatal("the invalid result became readable")
	}
}

func TestRegistryReceivesSequentialUpload(t *testing.T) {
	const owner = 1
	const allowance = 8
	r := New[int](Limits{MaxBytes: allowance, MaxRoutes: 1, UploadChunkBytes: 4})
	reserve := func() string {
		t.Helper()
		token, ok := r.ReserveUpload(owner, allowance)
		if !ok {
			t.Fatal("reserve sequential upload")
		}
		return token
	}
	chunk := func(offset uint32, contents string) *bytes.Reader {
		frame := binary.BigEndian.AppendUint32(nil, 7)
		frame = binary.BigEndian.AppendUint32(frame, offset)
		return bytes.NewReader(append(frame, contents...))
	}
	start := func(token string) {
		t.Helper()
		if err := r.ReceiveUpload(owner, token, chunk(0, "head")); err != nil {
			t.Fatal(err)
		}
	}
	token := reserve()
	start(token)
	if _, ready := r.TakeUpload(owner, token); ready {
		t.Fatal("partial result became readable")
	}
	if err := r.ReceiveUpload(owner+1, token, chunk(4, "end")); err != ErrGrant {
		t.Fatalf("another page could continue the upload: %v", err)
	}
	if err := r.ReceiveUpload(owner, token, chunk(4, "end")); err != nil {
		t.Fatal(err)
	}
	if data, ok := r.TakeUpload(owner, token); !ok || string(data) != "headend" {
		t.Fatalf("sequential result = %q, %v", data, ok)
	}

	token = reserve()
	start(token)
	if err := r.ReceiveUpload(owner, token, chunk(3, "end")); err != ErrSize {
		t.Fatalf("misplaced chunk returned %v", err)
	}
	token = reserve()
	start(token)
	r.RevokeOwner(owner)
	if err := r.ReceiveUpload(owner, token, chunk(4, "end")); err != ErrGrant {
		t.Fatalf("closed page could continue the upload: %v", err)
	}
	r.Revoke(owner, reserve())
}
