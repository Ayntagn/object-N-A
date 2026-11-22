// Package localassets grants an owner temporary access to in-memory resources.
package localassets

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"io"
	"sync"
)

// Limits bounds the bytes and grants retained by a Registry. An owner limit
// of zero uses the corresponding global limit. Global limits must be positive
// to grant resources.
type Limits struct {
	MaxBytes, MaxRoutes           int
	MaxOwnerBytes, MaxOwnerRoutes int
	// UploadChunkBytes enables sequential uploads with this many payload bytes
	// per request. The last request may be shorter. Zero uses a single request.
	UploadChunkBytes int
}

// Registry issues opaque grants. It is safe for concurrent requests.
type Registry[Owner comparable] struct {
	mu     sync.Mutex
	limits Limits
	bytes  int
	routes map[string]*record[Owner]
	owners map[Owner]usage
}

type usage struct{ bytes, routes int }

type state uint8

const (
	readable state = iota
	uploadReserved
	uploadReading
	uploadReady
	uploadRevoked
)

type record[Owner comparable] struct {
	owner    Owner
	state    state
	bytes    int
	received int
	asset    Asset
}

func New[Owner comparable](limits Limits) *Registry[Owner] {
	if limits.MaxOwnerBytes == 0 {
		limits.MaxOwnerBytes = limits.MaxBytes
	}
	if limits.MaxOwnerRoutes == 0 {
		limits.MaxOwnerRoutes = limits.MaxRoutes
	}
	return &Registry[Owner]{limits: limits, routes: make(map[string]*record[Owner]), owners: make(map[Owner]usage)}
}

// Asset is a snapshot of the approved bytes and their media type.
type Asset struct {
	mime string
	data []byte
	text string
}

func (a Asset) ContentType() string { return a.mime }

func (a Asset) Len() int { return len(a.data) + len(a.text) }

func (a Asset) WriteTo(w io.Writer) (int64, error) {
	var n int
	var err error
	if a.data != nil {
		n, err = w.Write(a.data)
	} else {
		n, err = io.WriteString(w, a.text)
	}
	if n < a.Len() && err == nil {
		err = io.ErrShortWrite
	}
	return int64(n), err
}

// AddBytes grants an immutable copy of data to owner.
func (r *Registry[Owner]) AddBytes(owner Owner, mime string, data []byte) (string, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	token, record := r.reserve(owner, len(data), readable)
	if record == nil {
		return "", false
	}
	record.asset = Asset{mime: mime, data: bytes.Clone(data)}
	return token, true
}

// AddText grants the provided string to owner.
func (r *Registry[Owner]) AddText(owner Owner, mime, text string) (string, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	token, record := r.reserve(owner, len(text), readable)
	if record == nil {
		return "", false
	}
	record.asset = Asset{mime: mime, text: text}
	return token, true
}

// Read resolves an active resource belonging to owner.
func (r *Registry[Owner]) Read(owner Owner, token string) (Asset, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	record := r.routes[token]
	if record == nil || record.owner != owner || record.state != readable {
		return Asset{}, false
	}
	return record.asset, true
}

// ReserveUpload issues one upload grant and reserves its full byte allowance.
func (r *Registry[Owner]) ReserveUpload(owner Owner, maxBytes int) (string, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	token, record := r.reserve(owner, maxBytes, uploadReserved)
	return token, record != nil
}

var (
	ErrGrant = errors.New("localassets: upload grant is unavailable")
	ErrSize  = errors.New("localassets: invalid upload size")
)

// ReceiveUpload accepts a four-byte big-endian total length. With chunking
// enabled, a four-byte big-endian offset follows, then one chunk of payload.
// Otherwise the request carries the entire payload. A grant allows one active
// read and is revoked if a request ends with an invalid or incomplete frame.
func (r *Registry[Owner]) ReceiveUpload(owner Owner, token string, body io.Reader) error {
	r.mu.Lock()
	record := r.routes[token]
	if record == nil || record.owner != owner || record.state != uploadReserved {
		r.mu.Unlock()
		return ErrGrant
	}
	record.state = uploadReading
	r.mu.Unlock()

	data, received, err := readUpload(body, record.bytes, r.limits.UploadChunkBytes, record.asset.data, record.received)
	r.mu.Lock()
	defer r.mu.Unlock()
	if record.state == uploadRevoked {
		r.remove(token, record)
		return ErrGrant
	}
	if err != nil {
		r.remove(token, record)
		return err
	}
	record.asset.data = data
	record.received = received
	record.state = uploadReserved
	if received == len(data) {
		record.state = uploadReady
	}
	return nil
}

// TakeUpload consumes the completed upload and transfers its bytes.
func (r *Registry[Owner]) TakeUpload(owner Owner, token string) ([]byte, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	record := r.routes[token]
	if record == nil || record.owner != owner || record.state != uploadReady {
		return nil, false
	}
	r.remove(token, record)
	return record.asset.data, true
}

// Revoke removes one grant belonging to owner.
func (r *Registry[Owner]) Revoke(owner Owner, token string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if record := r.routes[token]; record != nil && record.owner == owner {
		r.revoke(token, record)
	}
}

// RevokeOwner removes all grants belonging to owner.
func (r *Registry[Owner]) RevokeOwner(owner Owner) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for token, record := range r.routes {
		if record.owner == owner {
			r.revoke(token, record)
		}
	}
}

func (r *Registry[Owner]) reserve(owner Owner, count int, state state) (string, *record[Owner]) {
	u := r.owners[owner]
	if count <= 0 || count > r.limits.MaxBytes-r.bytes || count > r.limits.MaxOwnerBytes-u.bytes ||
		len(r.routes) >= r.limits.MaxRoutes || u.routes >= r.limits.MaxOwnerRoutes {
		return "", nil
	}
	token := rand.Text()
	for r.routes[token] != nil {
		token = rand.Text()
	}
	entry := &record[Owner]{owner: owner, state: state, bytes: count}
	r.routes[token] = entry
	r.owners[owner] = usage{u.bytes + count, u.routes + 1}
	r.bytes += count
	return token, entry
}

func (r *Registry[Owner]) revoke(token string, record *record[Owner]) {
	if record.state == uploadReading || record.state == uploadRevoked {
		// An in-flight upload retains its allowance until the read ends.
		record.state = uploadRevoked
		return
	}
	r.remove(token, record)
}

func (r *Registry[Owner]) remove(token string, record *record[Owner]) {
	delete(r.routes, token)
	r.bytes -= record.bytes
	u := r.owners[record.owner]
	if u.routes == 1 {
		delete(r.owners, record.owner)
	} else {
		r.owners[record.owner] = usage{u.bytes - record.bytes, u.routes - 1}
	}
}

func readUpload(body io.Reader, maxBytes, chunkBytes int, data []byte, received int) ([]byte, int, error) {
	if body == nil {
		return nil, 0, ErrSize
	}
	var header [8]byte
	headerBytes := 4
	if chunkBytes > 0 {
		headerBytes = len(header)
	}
	if _, err := io.ReadFull(body, header[:headerBytes]); err != nil {
		return nil, 0, err
	}
	size := binary.BigEndian.Uint32(header[:])
	offset := binary.BigEndian.Uint32(header[4:])
	if offset != uint32(received) || offset >= size || uint64(size) > uint64(maxBytes) ||
		(data != nil && uint64(size) != uint64(len(data))) {
		return nil, 0, ErrSize
	}
	if data == nil {
		data = make([]byte, int(size))
	}
	count := len(data) - received
	if chunkBytes > 0 {
		count = min(count, chunkBytes)
	}
	end := received + count
	if _, err := io.ReadFull(body, data[received:end]); err != nil {
		return nil, 0, err
	}
	if _, err := io.ReadFull(body, header[:1]); err != io.EOF {
		return nil, 0, ErrSize
	}
	return data, end, nil
}
