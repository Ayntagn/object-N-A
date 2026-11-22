package jpegn

// Entropy Decoding

// getHuffSymbol decodes a Huffman symbol from the bitstream without reading
// subsequent value bits.
func (d *decoder) getHuffSymbol(t *huffTable) int {
	if d.bufBits < 16 && !d.markerHit {
		d.showBits(16)
	}

	var look uint32
	if d.bufBits >= 16 {
		look = uint32((d.buf >> (d.bufBits - 16)) & 0xFFFF)
	} else if d.bufBits > 0 {
		sh := uint(16 - d.bufBits)
		look = uint32(((d.buf << sh) | ((uint64(1) << sh) - 1)) & 0xFFFF)
	} else {
		d.panic(ErrMissingHuffmanCode)
	}

	var huffBits int
	var huffCode uint8
	if e := t.lut[look>>8]; e != 0 {
		huffBits = int(e & 0xFF)
		huffCode = uint8(e >> 8)
	} else {
		var ok bool
		huffBits, huffCode, ok = t.decodeLong(look)
		if !ok {
			d.panic(ErrMissingHuffmanCode)
		}
	}

	if d.bufBits < huffBits {
		d.panic(ErrMissingHuffmanCode)
	}

	d.bufBits -= huffBits

	return int(huffCode)
}

// dequantLimit caps a dequantized coefficient; valid 8-bit images stay under 1024.
const dequantLimit = 1 << 13

// dequant scales a coefficient by its quantization factor, saturating.
func dequant(v int, q uint16) int32 {
	p := int64(v) * int64(q)

	if p > dequantLimit {
		return dequantLimit
	}

	if p < -dequantLimit {
		return -dequantLimit
	}

	return int32(p)
}

// decodeBlock decodes a single 8x8 block of a component (Baseline).
// This involves entropy decoding of DC and AC coefficients, dequantization, and applying the IDCT.
func (d *decoder) decodeBlock(c *component, outOffset int) {
	var code uint8
	var value int

	// This clears the array to zeros.
	d.block = [64]int32{}

	// Cache pointers to tables used in the loop.
	// Quantization table is stored in natural order (row-major), not zigzag.
	qt := d.qtab[c.qtSel]
	dcHuff := d.dcHuff[c.dcTabSel]
	acHuff := d.acHuff[c.acTabSel]

	// Decode DC coefficient.
	value = d.getVLC(dcHuff, nil)

	c.dcPred += value
	d.block[0] = dequant(c.dcPred, qt[0])

	// Decode AC coefficients.
	coef := 1 // coef is the zigzag index.
	for coef <= 63 {
		if d.bufBits < 16 && !d.markerHit {
			d.showBits(16)
		}

		var e int16
		if d.bufBits >= fastACBits {
			e = acHuff.fastAC[(d.buf>>uint(d.bufBits-fastACBits))&(1<<fastACBits-1)]
		}

		if e != 0 {
			d.bufBits -= int(e & 15)
			coef += int(e>>4) & 15
			value = int(e >> 8)
		} else {
			value = d.getVLC(acHuff, &code)

			if code == 0 { // EOB
				break
			}

			if (code & 0x0F) == 0 {
				if code != 0xF0 { // ZRL
					d.panic(ErrSyntax)
				}
				coef += 16
				if coef > 64 {
					d.panic(ErrSyntax)
				}

				continue
			}

			coef += int(code >> 4) // Skip run of zeros.
		}

		if coef > 63 {
			d.panic(ErrSyntax)
		}

		// Map zigzag index to natural index.
		naturalIndex := zz[coef]

		// Dequantize using the natural index.
		d.block[naturalIndex] = dequant(value, qt[naturalIndex])

		coef++
	}

	// Perform IDCT (in-place into the output pixel buffer).
	// Use scaled IDCT if scaling is enabled
	idctScaled(&d.block, c.pixels, outOffset, c.stride, d.scaleDenom)
}

// decodeScan decodes the image scan data. It dispatches to the baseline or progressive decoder.
// Handles panics from the hot path.
func (d *decoder) decodeScan() (err error) {
	// Recovery for panics in the hot path (getVLC, decodeBlock, progressive logic).
	defer func() {
		if r := recover(); r != nil {
			if de, ok := r.(errDecode); ok {
				err = de.error
			} else {
				// Propagate other panics
				panic(r)
			}
		}
	}()

	// decodeScanInternal parses the header and calls the appropriate baseline or progressive loop.
	return d.decodeScanInternal()
}

// scanParams is what an SOS header says about its scan.
type scanParams struct {
	n              int
	comp           [4]int
	ss, se, ah, al int
}

// decodeScanInternal handles the parsing of the SOS header and the main decoding loop.
func (d *decoder) decodeScanInternal() error {
	p, err := d.scanHeader()
	if err != nil {
		return err
	}

	return d.decodeScanBody(p)
}

// scanHeader parses an SOS header and leaves the bit reader at its entropy-coded data.
func (d *decoder) scanHeader() (scanParams, error) {
	// A SOF9 frame carrying Huffman tables is mislabelled and decodes fine, but
	// one without them is genuinely arithmetic coded and would come out as noise.
	if d.isArith && !d.sawDHT {
		return scanParams{}, ErrUnsupported
	}

	if err := d.decodeLength(); err != nil {
		return scanParams{}, err
	}

	if d.size < 1 {
		return scanParams{}, ErrSyntax
	}

	nCompScan := int(d.jpegData[d.pos])
	if d.length < (4+2*nCompScan) || nCompScan < 1 || nCompScan > 4 {
		return scanParams{}, ErrSyntax
	}

	// The fixed SOS header (count + 2 bytes/component + ss/se/ah/al) must be
	// present, not merely declared, before indexing into it.
	if d.size < 4+2*nCompScan {
		return scanParams{}, ErrSyntax
	}

	if err := d.skip(1); err != nil {
		return scanParams{}, err
	}

	qtNeeded := 0
	var scanComp [4]int

	for i := 0; i < nCompScan; i++ {
		scanID := int(d.jpegData[d.pos])

		found := false

		for j := 0; j < d.ncomp; j++ {
			if d.comp[j].id == scanID {
				c := &d.comp[j]
				scanComp[i] = j
				qtNeeded |= 1 << c.qtSel

				c.dcTabSel = int(d.jpegData[d.pos+1]) >> 4
				c.acTabSel = int(d.jpegData[d.pos+1]) & 0x0F

				if c.dcTabSel > 3 || c.acTabSel > 3 {
					return scanParams{}, ErrSyntax
				}

				found = true

				break
			}
		}

		if !found {
			return scanParams{}, ErrSyntax
		}

		if err := d.skip(2); err != nil {
			return scanParams{}, err
		}
	}

	if (qtNeeded &^ d.qtAvail) != 0 {
		return scanParams{}, ErrSyntax
	}

	ss := int(d.jpegData[d.pos])
	se := int(d.jpegData[d.pos+1])
	ah := int(d.jpegData[d.pos+2]) >> 4
	al := int(d.jpegData[d.pos+2]) & 0x0F

	if err := d.skip(d.length); err != nil {
		return scanParams{}, err
	}

	d.buf = 0
	d.bufBits = 0
	d.markerHit = false

	// Reset EOB run at the start of each scan, as required by the standard (Annex G.2.3).
	// This prevents excessive EOB runs from carrying over between scans.
	d.eobRun = 0

	return scanParams{n: nCompScan, comp: scanComp, ss: ss, se: se, ah: ah, al: al}, nil
}

// decodeScanBody decodes the entropy-coded data of a scan whose header p describes.
func (d *decoder) decodeScanBody(p scanParams) error {
	nCompScan, scanComp, ss, se, ah, al := p.n, p.comp, p.ss, p.se, p.ah, p.al

	if d.isBaseline {
		if ss != 0 || se != 63 || ah != 0 || al != 0 {
			return ErrUnsupported
		}

		return d.decodeScanBaseline(nCompScan, scanComp)
	}

	if ss > se || se > 63 {
		return ErrSyntax
	}

	if ah > 13 || al > 13 {
		return ErrSyntax
	}

	isDC := ss == 0
	if isDC {
		if se != 0 {
			return ErrSyntax
		}
	} else if nCompScan > 1 {
		return ErrSyntax
	}

	if isDC {
		return d.decodeScanProgressiveDC(nCompScan, scanComp, ah, al)
	}

	return d.decodeScanProgressiveAC(nCompScan, scanComp, ss, se, ah, al)
}

// decodeScanProgressiveDC handles progressive DC scans
func (d *decoder) decodeScanProgressiveDC(nCompScan int, scanComp [4]int, ah, al int) error {
	// Initialize restart interval counters
	rstCount := d.rstInterval
	nextRst := 0

	// Reset DC predictors for first pass
	if ah == 0 {
		for i := 0; i < nCompScan; i++ {
			d.comp[scanComp[i]].dcPred = 0
		}
	}

	// Single-component (non-interleaved) DC progressive scan path
	if nCompScan == 1 {
		compIndex := scanComp[0]
		c := &d.comp[compIndex]
		blocksProcessed := 0
		totalBlocks := c.blocksPerLine * c.blocksPerCol

		// Non-interleaved: iterate the true block raster (no MCU padding),
		// addressing the buffer with the nBlocksX stride.
		for by := 0; by < c.blocksPerCol; by++ {
			for bx := 0; bx < c.blocksPerLine; bx++ {
				coeffOffset := (by*c.nBlocksX + bx) * 64

				// Bounds check
				if coeffOffset+64 > len(c.coeffs) {
					return ErrInternal
				}

				// Decode DC coefficient
				if ah == 0 {
					// First pass - decode DC coefficient difference
					var diff int

					// Try to decode the DC coefficient
					diff = d.getVLC(d.dcHuff[c.dcTabSel], nil)

					// Always update predictor and store coefficient
					c.dcPred += diff
					c.coeffs[coeffOffset] = int32(c.dcPred) << al
				} else {
					// Refinement pass.
					bit := d.getBit()
					if bit > 0 {
						c.coeffs[coeffOffset] |= int32(1 << al)
					}

				}

				blocksProcessed++
				if d.rstInterval > 0 && blocksProcessed < totalBlocks {
					rstCount--
					if rstCount == 0 {
						if err := d.consumeRestart(&nextRst); err != nil {
							return err
						}
						rstCount = d.rstInterval
					}
				}
			}
		}

		return d.finishScan()
	}

	// Interleaved scan path (multiple components)
	mcu, totalMCUs := 0, d.mbWidth*d.mbHeight
	for mby := 0; mby < d.mbHeight; mby++ {
		for mbx := 0; mbx < d.mbWidth; mbx++ {
			// Process all components for this MCU
			for i := 0; i < nCompScan; i++ {
				compIndex := scanComp[i]
				c := &d.comp[compIndex]

				for sby := 0; sby < c.ssY; sby++ {
					for sbx := 0; sbx < c.ssX; sbx++ {
						by := mby*c.ssY + sby
						bx := mbx*c.ssX + sbx

						if by >= c.nBlocksY || bx >= c.nBlocksX {
							return ErrInternal
						}

						blockIndex := by*c.nBlocksX + bx
						coeffOffset := blockIndex * 64

						if coeffOffset+64 > len(c.coeffs) {
							return ErrInternal
						}

						// Process the block
						if ah == 0 {
							// First pass - decode DC coefficient difference
							diff := d.getVLC(d.dcHuff[c.dcTabSel], nil)

							// Update predictor and store coefficient
							c.dcPred += diff
							c.coeffs[coeffOffset] = int32(c.dcPred) << al
						} else {
							// Refinement pass
							bit := d.getBit()
							if bit > 0 {
								c.coeffs[coeffOffset] |= int32(1 << al)
							}
						}
					}
				}
			}

			mcu++
			if d.rstInterval > 0 && mcu < totalMCUs {
				rstCount--
				if rstCount == 0 {
					if err := d.consumeRestart(&nextRst); err != nil {
						return err
					}
					rstCount = d.rstInterval
				}
			}
		}
	}

	return d.finishScan()
}

// decodeScanBaseline decodes a baseline JPEG scan.
func (d *decoder) decodeScanBaseline(nCompScan int, scanComp [4]int) error {
	// Reset DC predictors at the start of the scan.
	for k := 0; k < d.ncomp; k++ {
		d.comp[k].dcPred = 0
	}

	// Reset EOB run for baseline.
	d.eobRun = 0

	for i := 0; i < nCompScan; i++ {
		d.scanned |= 1 << uint(scanComp[i])
	}

	// Calculate output block dimensions based on scaling
	outBlockW := 8 / d.scaleDenom
	outBlockH := 8 / d.scaleDenom

	var total int

	var decodeUnit func(i int)

	if nCompScan == 1 {
		// The component's own block raster, addressed through the padded stride.
		c := &d.comp[scanComp[0]]
		total = c.blocksPerLine * c.blocksPerCol

		decodeUnit = func(i int) {
			by, bx := i/c.blocksPerLine, i%c.blocksPerLine
			d.decodeBlock(c, by*outBlockH*c.stride+bx*outBlockW)
		}
	} else {
		total = d.mbWidth * d.mbHeight

		decodeUnit = func(i int) {
			mbx, mby := i%d.mbWidth, i/d.mbWidth

			for k := 0; k < nCompScan; k++ {
				c := &d.comp[scanComp[k]]

				for sby := 0; sby < c.ssY; sby++ {
					for sbx := 0; sbx < c.ssX; sbx++ {
						rowStart := (mby*c.ssY + sby) * outBlockH * c.stride
						colStart := (mbx*c.ssX + sbx) * outBlockW
						d.decodeBlock(c, rowStart+colStart)
					}
				}
			}
		}
	}

	if d.rstInterval > 0 {
		if err := d.decodeScanBaselineRestart(total, decodeUnit); err != nil {
			return err
		}
	} else {
		for i := 0; i < total; i++ {
			decodeUnit(i)
		}
	}

	return d.finishScan()
}

// decodeScanBaselineRestart checks the marker between complete intervals.
func (d *decoder) decodeScanBaselineRestart(total int, decodeUnit func(i int)) error {
	nextRst := 0

	for mcu := 0; mcu < total; {
		end := min(total, mcu+d.rstInterval)
		for ; mcu < end; mcu++ {
			decodeUnit(mcu)
		}

		if mcu < total {
			if err := d.consumeRestart(&nextRst); err != nil {
				return err
			}
		}
	}
	return nil
}

// decodeScanProgressiveAC handles an AC scan in a progressive JPEG.
func (d *decoder) decodeScanProgressiveAC(nCompScan int, scanComp [4]int, ss, se, ah, al int) error {
	// AC scans must be non-interleaved (nCompScan=1).
	if nCompScan != 1 {
		return ErrInternal
	}

	rstCount := d.rstInterval
	nextRst := 0

	var decodeFunc func(*component, int, int, int, int, int, int)
	if ah == 0 {
		decodeFunc = d.decodeBlockACFirst
	} else {
		decodeFunc = d.decodeBlockACRefine
	}

	c := &d.comp[scanComp[0]]

	// Non-interleaved: iterate the true block raster, addressing with the nBlocksX stride.
	totalBlocks := c.blocksPerLine * c.blocksPerCol

	blockIndex := 0
	for by := 0; by < c.blocksPerCol; by++ {
		for bx := 0; bx < c.blocksPerLine; bx++ {
			coeffOffset := (by*c.nBlocksX + bx) * 64

			// Bounds check for coefficient buffer
			if coeffOffset+64 > len(c.coeffs) {
				return ErrInternal
			}

			// Check if we have an active EOB run
			if d.eobRun > 0 {
				// Apply EOB run - this block gets EOB processing
				if ah > 0 {
					d.refineBlockEOB(c, coeffOffset, ss, se, al)
				}
				// For first pass (ah == 0), coefficients remain zero (already initialized)

				d.eobRun--

				// Do NOT use 'continue' here. We must fall through to the restart marker check.
			} else {
				// No active EOB run - try to decode the block.
				remainingBlocks := totalBlocks - blockIndex

				decodeFunc(c, coeffOffset, ss, se, al, blockIndex, remainingBlocks)

			}

			blockIndex++

			// Handle restart markers (checked after every block)
			if d.rstInterval > 0 && blockIndex < totalBlocks {
				rstCount--
				if rstCount == 0 {
					// AC scans are always single component (nCompScan=1).
					if err := d.consumeRestart(&nextRst); err != nil {
						return err
					}
					rstCount = d.rstInterval
				}
			}
		}
	}

	return d.finishScan()
}

// decodeBlockACFirst handles the first pass (Ah=0) AC coefficient decoding.
func (d *decoder) decodeBlockACFirst(c *component, offset, ss, se, al, blockIndex, remainingBlocks int) {
	// Check bounds before accessing coeffs
	if offset+64 > len(c.coeffs) {
		d.panic(ErrInternal)
	}

	acHuff := d.acHuff[c.acTabSel]
	coefs := c.coeffs[offset : offset+64 : offset+64]

	for k := ss; k <= se; {
		// Ultra-fast path: 8-bit look-ahead, then the long-code path.
		var symbol int

		if d.bufBits >= 8 && !d.markerHit {
			value8 := (d.buf >> (d.bufBits - 8)) & 0xFF
			if entry := acHuff.lut[value8]; entry != 0 {
				d.bufBits -= int(entry & 0xFF)
				symbol = int(entry >> 8)
			} else {
				symbol = d.getHuffSymbol(acHuff)
			}
		} else {
			symbol = d.getHuffSymbol(acHuff)
		}

		s := symbol & 0x0F
		r := symbol >> 4

		if s == 0 {
			if r == 15 { // ZRL (Zero Run Length)
				k += 16
				if k > se+1 {
					d.panic(ErrSyntax)
				}
				continue
			}

			// EOB (End of Block) run
			run := 1 << r
			if r > 0 {
				run += d.getBits(int(r))
			}

			if run > remainingBlocks {
				d.panic(ErrSyntax)
			}

			// The run includes the current block, so store run-1.
			d.eobRun = run - 1

			return
		}

		k += r
		if k > se {
			d.panic(ErrSyntax)
		}

		// Read coefficient value
		val := d.getBits(s)

		if val < (1 << (s - 1)) {
			val += ((-1) << s) + 1
		}

		nat := zz[k]
		coefs[nat] = int32(val << al)

		k++
	}
}

// decodeBlockACRefine handles the refinement pass (Ah>0) AC coefficient decoding.
func (d *decoder) decodeBlockACRefine(c *component, offset, ss, se, al, blockIndex, remainingBlocks int) {
	// Check bounds before accessing coeffs
	if offset+64 > len(c.coeffs) {
		d.panic(ErrInternal)
	}

	acHuff := d.acHuff[c.acTabSel]
	delta := int32(1 << al)
	coefs := c.coeffs[offset : offset+64 : offset+64]

	if err := d.refineBlock(coefs, acHuff, int32(ss), int32(se), delta, blockIndex, remainingBlocks, false); err != nil {
		d.panic(err)
	}
}

// refineBlock implements progressive AC refinement working directly on coefficient slice.
func (d *decoder) refineBlock(coefs []int32, t *huffTable, zigStart, zigEnd, delta int32, blockIndex, remainingBlocks int, trace bool) error {
	zig := zigStart

	// This block is not in an EOB run, so decode its coefficient data.
	newCoeffs := 0
	refinements := 0

loop:
	for zig <= zigEnd {
		z := int32(0)

		// Ultra-fast path with 8-bit lookup
		var symbol int

		if d.bufBits >= 8 && !d.markerHit {
			value8 := (d.buf >> (d.bufBits - 8)) & 0xFF
			if entry := t.lut[value8]; entry != 0 {
				d.bufBits -= int(entry & 0xFF)
				symbol = int(entry >> 8)
			} else {
				symbol = d.getHuffSymbol(t)
			}
		} else {
			symbol = d.getHuffSymbol(t)
		}

		val0 := int32(symbol >> 4)   // run of zeroes
		val1 := int32(symbol & 0x0f) // category/size

		switch val1 {
		case 0: // EOB or ZRL
			if val0 != 0x0f { // EOB
				eobRun := int32(1) << val0
				if val0 > 0 {
					eobRun += int32(d.getBits(int(val0)))
				}

				maxRun := int32(remainingBlocks)
				if eobRun > maxRun {
					return ErrSyntax
				}

				d.eobRun = int(eobRun - 1)

				break loop
			}
			// ZRL - fall through to refine existing coefficients
		case 1: // New non-zero coefficient
			bit := d.getBit()
			z = delta
			if bit == 0 {
				z = -z
			}

			newCoeffs++
		default:
			return ErrSyntax
		}

		// R counts zero coefficients to skip; non-zero ones are refined in passing.
		nz := val0
		for zig <= zigEnd {
			u := zz[zig]
			if coefs[u] == 0 {
				if nz == 0 {
					// Found target zero position - place coefficient here
					break
				}
				nz--
			} else {
				// Refine existing non-zero coefficient
				bit := d.getBit()
				if bit != 0 {
					if coefs[u] >= 0 {
						coefs[u] += delta
					} else {
						coefs[u] -= delta
					}
					refinements++
				}
			}
			zig++
		}

		if zig > zigEnd {
			return ErrSyntax
		}

		// Set new coefficient if we have one
		if z != 0 {
			coefs[zz[zig]] = z
		}

		zig++
	}

	// Refine any remaining non-zero coefficients in this block (if loop broke due to EOB run starting)
	// This refinement must happen before the EOB run takes effect for the subsequent blocks.
	for zig <= zigEnd {
		u := zz[zig]
		if coefs[u] != 0 {
			bit := d.getBit()
			if bit != 0 {
				if coefs[u] >= 0 {
					coefs[u] += delta
				} else {
					coefs[u] -= delta
				}
				refinements++
			}
		}
		zig++
	}

	return nil
}

// refineBlockEOB refines existing non-zero coefficients in a block during EOB run processing
func (d *decoder) refineBlockEOB(c *component, offset, ss, se, al int) {
	if offset+64 > len(c.coeffs) {
		d.panic(ErrInternal)
	}

	coefs := c.coeffs[offset : offset+64 : offset+64]
	delta := int32(1 << al)

	// Refine existing non-zero coefficients in the spectral range
	for k := ss; k <= se; k++ {
		nat := zz[k]
		if coefs[nat] != 0 {
			bit := d.getBit()
			if bit != 0 {
				if coefs[nat] >= 0 {
					coefs[nat] += delta
				} else {
					coefs[nat] -= delta
				}
			}
		}
	}
}

// postProcessProgressive performs dequantization and IDCT after all progressive scans are complete.
func (d *decoder) postProcessProgressive() error {
	// Calculate output block dimensions based on scaling
	outBlockW := 8 / d.scaleDenom
	outBlockH := 8 / d.scaleDenom

	for i := 0; i < d.ncomp; i++ {
		c := &d.comp[i]
		qt := d.qtab[c.qtSel]

		// Stride already computed in decodeSOF with scaled block width
		// c.stride = c.nBlocksX * outBlockW

		// Use c.nBlocksY * outBlockH as the padded height for scaled output
		paddedHeight := c.nBlocksY * outBlockH
		pixelSize := c.stride * paddedHeight

		if c.pixels == nil && pixelSize > 0 {
			c.pixels = make([]byte, pixelSize)
		}

		if len(c.coeffs) == 0 {
			continue
		}

		// Process all blocks using 2D iteration to avoid division
		blocksProcessed := 0

		for by := 0; by < c.nBlocksY; by++ {
			pixelY := by * outBlockH

			// Skip entire row if outside component height
			if pixelY >= c.height {
				continue
			}

			for bx := 0; bx < c.nBlocksX; bx++ {
				pixelX := bx * outBlockW

				// Skip blocks outside component width
				if pixelX >= c.width {
					continue
				}

				// Use simple linear coefficient indexing
				blockIndex := by*c.nBlocksX + bx
				coeffOffset := blockIndex * 64

				// Bounds check for coefficient buffer
				if coeffOffset+64 > len(c.coeffs) {
					continue
				}

				// Clear work block and dequantize
				// Branchless dequant; writing all 64 also zero-fills, no clear needed.
				coefs := c.coeffs[coeffOffset : coeffOffset+64]
				for k := 0; k < 64; k++ {
					d.block[k] = dequant(int(coefs[k]), qt[k])
				}

				// Apply IDCT to pixel buffer
				if len(c.pixels) > 0 {
					outOffset := pixelY*c.stride + pixelX

					// Bounds check for pixel buffer
					if outOffset >= 0 && outOffset < len(c.pixels) &&
						pixelY < paddedHeight && pixelX < c.stride { // Use padded bounds for safety
						// Apply scaled IDCT
						idctScaled(&d.block, c.pixels, outOffset, c.stride, d.scaleDenom)
					}
				}

				blocksProcessed++
			}
		}

		// Free coefficients to save memory
		c.coeffs = nil
	}

	return nil
}
