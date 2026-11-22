// spec: EV-UI-290 / EV-MYGO-195 / EV-MYGO-326
package vaultcontext

import (
	"bytes"
	"context"
	"reflect"
	"runtime"
	"testing"
	"weak"
)

func TestPackedIndexKeepsExactRetrieval(t *testing.T) {
	index, query := retrievalFixture(32, 16)
	ctx := context.Background()
	want, err := index.SearchVector(ctx, query)
	if err != nil {
		t.Fatal(err)
	}
	var data bytes.Buffer
	if err := index.WritePacked(&data); err != nil {
		t.Fatal(err)
	}
	restored, err := ReadPacked(bytes.NewReader(data.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	got, err := restored.SearchVector(ctx, query)
	if err != nil || len(want) != searchLimit || !reflect.DeepEqual(got, want) {
		t.Fatalf("reopened ranking = %#v, %v, want %#v", got, err, want)
	}
	corrupt := bytes.Clone(data.Bytes())
	corrupt[len(corrupt)-33] ^= 1
	if _, err := ReadPacked(bytes.NewReader(corrupt)); err == nil {
		t.Fatal("accepted a damaged packed vector")
	}
}

func TestPrunePackedIndexReleasesDiscardedVectors(t *testing.T) {
	pruned, discarded := func() (*Index, weak.Pointer[float32]) {
		index, _ := retrievalFixture(8, 1024)
		var data bytes.Buffer
		if err := index.WritePacked(&data); err != nil {
			t.Fatal(err)
		}
		opened, err := ReadPacked(&data)
		if err != nil {
			t.Fatal(err)
		}
		discarded := weak.Make(&opened.data.Chunks[7].Vector[0])
		pruned, err := Prune(context.Background(), opened, opened.Paths()[:1])
		if err != nil || pruned.ChunkCount() != 4 {
			t.Fatalf("expected only the first note's chunks: %v, %v", pruned, err)
		}
		return pruned, discarded
	}()
	runtime.GC()
	if discarded.Value() != nil {
		t.Error("retained note keeps a removed note's vectors alive")
	}
	runtime.KeepAlive(pruned)
}
