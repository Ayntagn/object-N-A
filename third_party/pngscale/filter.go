// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pngscale

// unfilter reconstructs a PNG scanline in place. The stride is one for packed
// samples and is the size of a complete pixel in bytes for other samples.
func unfilter(filter byte, row, previous []byte, stride int) error {
	switch filter {
	case 0:
	case 1:
		for i := stride; i < len(row); i++ {
			row[i] += row[i-stride]
		}
	case 2:
		for i, above := range previous {
			row[i] += above
		}
	case 3:
		for i := 0; i < stride; i++ {
			row[i] += previous[i] / 2
		}
		for i := stride; i < len(row); i++ {
			row[i] += byte((int(row[i-stride]) + int(previous[i])) / 2)
		}
	case 4:
		filterPaeth(row, previous, stride)
	default:
		return FormatError("invalid scanline filter")
	}
	return nil
}

func abs(x int) int {
	// If x is negative, flip the bits and add one without a branch.
	m := x >> 31
	return (x ^ m) - m
}

// filterPaeth is adapted from Go's image/png/paeth.go. Iterating over each
// channel separately keeps the left and upper-left samples in registers.
func filterPaeth(row, previous []byte, stride int) {
	for i := 0; i < stride; i++ {
		left, upperLeft := 0, 0
		for j := i; j < len(row); j += stride {
			above := int(previous[j])
			da := above - upperLeft
			db := left - upperLeft
			dc := abs(da + db)
			da, db = abs(da), abs(db)
			if da <= db && da <= dc {
				// The predictor is already in left.
			} else if db <= dc {
				left = above
			} else {
				left = upperLeft
			}
			left = (left + int(row[j])) & 0xff
			row[j] = byte(left)
			upperLeft = above
		}
	}
}
