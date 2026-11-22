package vaultcontext

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"io"
	"math"
	"slices"
)

const packedIndexHeader = "ONAVECP\x01"
const maxPackedNoteVectors = 128

// WritePacked writes a cache snapshot with the metadata followed by float32
// vectors in their original order. A digest covers the metadata and vectors.
func (i *Index) WritePacked(dst io.Writer) error {
	metadata := i.data
	metadata.Chunks = slices.Clone(metadata.Chunks)
	for at := range metadata.Chunks {
		metadata.Chunks[at].Vector = nil
	}
	encoded, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	buffer := bufio.NewWriterSize(dst, 32<<10)
	if _, err := buffer.WriteString(packedIndexHeader); err != nil {
		return err
	}
	digest := sha256.New()
	writer := io.MultiWriter(buffer, digest)
	if err := binary.Write(writer, binary.LittleEndian, uint64(len(encoded))); err != nil {
		return err
	}
	if _, err := writer.Write(encoded); err != nil {
		return err
	}
	row := make([]byte, 4*i.data.Dimension)
	for _, chunk := range i.data.Chunks {
		if len(chunk.Vector) != i.data.Dimension {
			return errIndexFormat
		}
		_, err := binary.Encode(row, binary.LittleEndian, chunk.Vector)
		if err != nil {
			return err
		}
		if _, err := writer.Write(row); err != nil {
			return err
		}
	}
	if _, err := buffer.Write(digest.Sum(nil)); err != nil {
		return err
	}
	return buffer.Flush()
}

// ReadPacked loads a cache snapshot and validates it before returning an
// immutable Index. Old JSON snapshots are read with UnmarshalJSON.
func ReadPacked(src io.Reader) (*Index, error) {
	var magic [len(packedIndexHeader)]byte
	if _, err := io.ReadFull(src, magic[:]); err != nil {
		return nil, err
	}
	if string(magic[:]) != packedIndexHeader {
		return nil, errIndexFormat
	}
	digest := sha256.New()
	reader := io.TeeReader(src, digest)
	var metadataBytes uint64
	if err := binary.Read(reader, binary.LittleEndian, &metadataBytes); err != nil {
		return nil, err
	}
	if metadataBytes > math.MaxInt {
		return nil, errIndexFormat
	}
	// Grow in response to the bytes actually read rather than the declared
	// length in a possibly truncated cache.
	var encoded bytes.Buffer
	if _, err := io.CopyN(&encoded, reader, int64(metadataBytes)); err != nil {
		return nil, err
	}
	var data storedIndex
	if err := json.Unmarshal(encoded.Bytes(), &data); err != nil {
		return nil, err
	}
	if data.Version != indexVersion || data.Dimension < 0 || data.Dimension > maxDimensions ||
		(data.Dimension == 0 && len(data.Chunks) != 0) {
		return nil, errIndexFormat
	}
	row := make([]byte, 4*data.Dimension)
	for from := 0; from < len(data.Chunks); {
		// Prune and Update retain or remove whole notes. A batch can only hold
		// chunks from one note so it can be released with that note.
		file := data.Chunks[from].File
		limit := min(maxPackedNoteVectors, len(data.Chunks)-from)
		count := 1
		for count < limit && data.Chunks[from+count].File == file {
			count++
		}
		values := make([]float32, count*data.Dimension)
		chunks := data.Chunks[from : from+count]
		for at := range chunks {
			chunk := &chunks[at]
			if chunk.Vector != nil {
				return nil, errIndexFormat
			}
			if _, err := io.ReadFull(reader, row); err != nil {
				return nil, err
			}
			end := (at + 1) * data.Dimension
			vector := values[at*data.Dimension : end : end]
			if _, err := binary.Decode(row, binary.LittleEndian, vector); err != nil {
				return nil, err
			}
			chunk.setVector(vector)
		}
		from += count
	}
	var checksum [sha256.Size]byte
	if _, err := io.ReadFull(src, checksum[:]); err != nil {
		return nil, err
	}
	if !bytes.Equal(checksum[:], digest.Sum(nil)) || !data.valid() {
		return nil, errIndexFormat
	}
	return &Index{data: data}, nil
}
