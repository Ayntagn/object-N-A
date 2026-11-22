// spec: EV-UI-290 / EV-MYGO-195
package vectorsearch

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestCosineTopKPreservesThresholdAndInputOrder(t *testing.T) {
	values := [][]float32{{0, 0}, {3, 4}, {6, 8}, {1, 0}, {0, 1}, {9, 12}}
	row := func(i int) Vector { return Vector{Values: values[i], Norm: Norm(values[i])} }
	got, err := CosineTopK(context.Background(), []float32{1, 0}, len(values), 3, 0.6, row)
	if err != nil {
		t.Fatal(err)
	}
	want := []Match{{Index: 3, Score: 1}, {Index: 1, Score: 0.6}, {Index: 2, Score: 0.6}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("top matches = %v, want %v", got, want)
	}
	ctx, cancel := context.WithCancel(context.Background())
	got, err = CosineTopK(ctx, []float32{1, 0}, len(values), 3, 0.6, func(i int) Vector {
		if i == 2 {
			cancel()
		}
		return row(i)
	})
	if !errors.Is(err, context.Canceled) || got != nil {
		t.Fatalf("cancellation = %v, %v", got, err)
	}
}
