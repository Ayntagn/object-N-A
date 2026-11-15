package ui

import (
	"errors"
	"strings"

	"github.com/ebitengine/purego/objc"
)

const documentTitleMetadataSupported = true

const documentTitleTagsKey = "NSURLTagNamesKey"
const documentTitleLockedKey = "NSURLIsUserImmutableKey"

func readDocumentTitleMetadata(absolute string) ([]string, bool, error) {
	defer nativeAutoreleasePool()()
	url := documentTitleFileURL(absolute)
	var names, locked objc.ID
	if !documentTitleResourceValue(url, documentTitleTagsKey, &names) ||
		!documentTitleResourceValue(url, documentTitleLockedKey, &locked) {
		return nil, false, errors.New("Could not read the file's metadata")
	}
	var tags []string
	for i, size := uint(0), objc.Send[uint](names, objc.RegisterName("count")); i < size; i++ {
		name := names.Send(objc.RegisterName("objectAtIndex:"), i)
		tags = append(tags, strings.Clone(objc.Send[string](name, objc.RegisterName("UTF8String"))))
	}
	return tags, objc.Send[bool](locked, objc.RegisterName("boolValue")), nil
}

func writeDocumentTitleTags(absolute string, tags []string) error {
	defer nativeAutoreleasePool()()
	values := objc.ID(objc.GetClass("NSMutableArray")).Send(objc.RegisterName("arrayWithCapacity:"), uint(len(tags)))
	for _, tag := range tags {
		values.Send(objc.RegisterName("addObject:"), nativeNSString(tag))
	}
	if !setDocumentTitleResourceValue(absolute, documentTitleTagsKey, values) {
		return errors.New("Could not update Finder tags")
	}
	return nil
}

func writeDocumentTitleLock(absolute string, locked bool) error {
	defer nativeAutoreleasePool()()
	value := objc.ID(objc.GetClass("NSNumber")).Send(objc.RegisterName("numberWithBool:"), locked)
	if !setDocumentTitleResourceValue(absolute, documentTitleLockedKey, value) {
		return errors.New("Could not change the file lock")
	}
	return nil
}

func documentTitleFileURL(absolute string) objc.ID {
	return objc.ID(objc.GetClass("NSURL")).Send(objc.RegisterName("fileURLWithPath:"), nativeNSString(absolute))
}

func documentTitleResourceValue(url objc.ID, key string, value *objc.ID) bool {
	return objc.Send[bool](url, objc.RegisterName("getResourceValue:forKey:error:"), value, nativeNSString(key), uintptr(0))
}

func setDocumentTitleResourceValue(absolute, key string, value objc.ID) bool {
	return objc.Send[bool](documentTitleFileURL(absolute), objc.RegisterName("setResourceValue:forKey:error:"),
		value, nativeNSString(key), uintptr(0))
}
