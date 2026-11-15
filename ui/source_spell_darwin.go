package ui

import "github.com/ebitengine/purego/objc"

const sourceSpellAvailable = true

func requestSystemSourceSpelling(text string, complete func([]sourceSpellingRange)) {
	defer nativeAutoreleasePool()()
	checker := objc.ID(objc.GetClass("NSSpellChecker")).Send(objc.RegisterName("sharedSpellChecker"))
	const utf8Encoding = uint(4) // NSUTF8StringEncoding
	input := objc.ID(objc.GetClass("NSString")).Send(objc.RegisterName("alloc")).
		Send(objc.RegisterName("initWithBytes:length:encoding:"), text, uint(len(text)), utf8Encoding)
	defer input.Send(objc.RegisterName("release"))
	if checker == 0 || input == 0 {
		complete(nil)
		return
	}
	handler := objc.NewBlock(func(_ objc.Block, _ int, results, _ objc.ID, _ int) {
		count := objc.Send[uint](results, objc.RegisterName("count"))
		ranges := make([]sourceSpellingRange, 0, count)
		for index := range count {
			result := results.Send(objc.RegisterName("objectAtIndex:"), index)
			ranges = append(ranges, objc.Send[sourceSpellingRange](result, objc.RegisterName("range")))
		}
		complete(ranges)
	})
	defer handler.Release()
	checkRange := sourceSpellingRange{Length: objc.Send[uint](input, objc.RegisterName("length"))}
	const spellingType = uint(1 << 1) // NSTextCheckingTypeSpelling
	checker.Send(objc.RegisterName("requestCheckingOfString:range:types:options:inSpellDocumentWithTag:completionHandler:"),
		input, checkRange, spellingType, objc.ID(0), 0, handler)
}
