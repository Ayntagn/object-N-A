package ui

import (
	"runtime"
	"strings"
	"sync"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
)

type pdfFontLookup struct {
	createGraphicsFont func(objc.ID) uintptr
	createTextFont     func(uintptr, float64, uintptr, uintptr) uintptr
	copyPostScriptName func(uintptr) objc.ID
	glyphs             func(uintptr, *uint16, *uint16, int) bool
	release            func(uintptr)
}

var loadPDFFontLookup = sync.OnceValue(func() *pdfFontLookup {
	handle, err := purego.Dlopen("/System/Library/Frameworks/CoreText.framework/CoreText", purego.RTLD_LAZY|purego.RTLD_LOCAL)
	if err != nil {
		return nil
	}
	var lookup pdfFontLookup
	for _, binding := range []struct {
		name string
		fn   any
	}{
		{"CGFontCreateWithFontName", &lookup.createGraphicsFont},
		{"CTFontCreateWithGraphicsFont", &lookup.createTextFont},
		{"CTFontCopyPostScriptName", &lookup.copyPostScriptName},
		{"CTFontGetGlyphsForCharacters", &lookup.glyphs},
		{"CFRelease", &lookup.release},
	} {
		symbol, err := purego.Dlsym(handle, binding.name)
		if err != nil {
			_ = purego.Dlclose(handle)
			return nil
		}
		purego.RegisterFunc(binding.fn, symbol)
	}
	return &lookup
})

// Match the installed font's glyphs, following MyGo v0.8.3's PrintToPDF
// correction for CoreGraphics ToUnicode mappings.
func pdfUnicodeReplacements(radicals map[string][]uint16) map[string]map[uint16]uint16 {
	if len(radicals) == 0 {
		return nil
	}
	lookup := loadPDFFontLookup()
	if lookup == nil {
		return nil
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	defer nativeAutoreleasePool()()

	replacements := make(map[string]map[uint16]uint16)
	for name, chars := range radicals {
		if name == "" || strings.ContainsRune(name, '\x00') || len(chars) == 0 {
			continue
		}
		if found := lookup.replacements(name, chars); len(found) > 0 {
			replacements[name] = found
		}
	}
	return replacements
}

func (lookup *pdfFontLookup) replacements(name string, radicals []uint16) map[uint16]uint16 {
	// CoreGraphics can resolve the system UI fonts whose PostScript names
	// begin with a dot, such as the PingFang fonts in WKWebView PDF output.
	fontName := nativeNSString(name)
	if fontName == 0 {
		return nil
	}
	graphicsFont := lookup.createGraphicsFont(fontName)
	if graphicsFont == 0 {
		return nil
	}
	defer lookup.release(graphicsFont)
	font := lookup.createTextFont(graphicsFont, 12, 0, 0)
	if font == 0 {
		return nil
	}
	defer lookup.release(font)

	postScriptName := lookup.copyPostScriptName(font)
	if postScriptName == 0 {
		return nil
	}
	defer lookup.release(uintptr(postScriptName))
	if objc.Send[string](postScriptName, objc.RegisterName("UTF8String")) != name {
		return nil
	}

	glyphs := make([]uint16, len(radicals))
	lookup.glyphs(font, &radicals[0], &glyphs[0], len(glyphs))
	wanted := make(map[uint16][]uint16)
	for index, glyph := range glyphs {
		char := radicals[index]
		if glyph != 0 && char >= 0x2E80 && char <= 0x2FDF {
			wanted[glyph] = append(wanted[glyph], char)
		}
	}

	found := make(map[uint16]uint16)
	// Use the first matching CJK unified ideograph, then search Extension A.
	const unifiedFirst, unifiedLast = 0x4E00, 0x9FFF
	var chars, candidates [unifiedLast - unifiedFirst + 1]uint16
	for _, block := range [2]struct{ first, last uint16 }{{unifiedFirst, unifiedLast}, {0x3400, 0x4DBF}} {
		if len(wanted) == 0 {
			break
		}
		count := int(block.last-block.first) + 1
		for index := range count {
			chars[index] = block.first + uint16(index)
		}
		lookup.glyphs(font, &chars[0], &candidates[0], count)
		for index, glyph := range candidates[:count] {
			if aliases := wanted[glyph]; len(aliases) > 0 {
				for _, alias := range aliases {
					found[alias] = chars[index]
				}
				delete(wanted, glyph)
			}
		}
	}
	return found
}
