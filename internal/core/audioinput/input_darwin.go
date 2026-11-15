package audioinput

import (
	"fmt"
	"unsafe"

	"github.com/ebitengine/purego"
)

const (
	audioSystemObject          = 1
	audioPropertyDevices       = 0x64657623 // 'dev#'
	audioPropertyStreams       = 0x73746d23 // 'stm#'
	audioPropertyDeviceUID     = 0x75696420 // 'uid '
	audioPropertyName          = 0x6c6e616d // 'lnam'
	audioScopeGlobal           = 0x676c6f62 // 'glob'
	audioScopeInput            = 0x696e7074 // 'inpt'
	audioBadPropertySize       = 0x2173697a // '!siz'
	coreFoundationEncodingUTF8 = 0x08000100
)

type audioObjectPropertyAddress struct {
	selector uint32
	scope    uint32
	element  uint32 // Zero is the main element.
}

type cfRange struct {
	location int
	length   int
}

type coreAudioProperties struct {
	dataSize func(uint32, *audioObjectPropertyAddress, uint32, unsafe.Pointer, *uint32) int32
	data     func(uint32, *audioObjectPropertyAddress, uint32, unsafe.Pointer, *uint32, unsafe.Pointer) int32
	length   func(uintptr) int
	getBytes func(uintptr, cfRange, uint32, uint8, uint8, *byte, int, *int) int
	release  func(uintptr)
}

// List returns the devices with input streams in CoreAudio's order.
func List() ([]Device, error) {
	audio, err := purego.Dlopen("/System/Library/Frameworks/CoreAudio.framework/CoreAudio", purego.RTLD_LAZY|purego.RTLD_LOCAL)
	if err != nil {
		return nil, fmt.Errorf("load CoreAudio: %w", err)
	}
	defer purego.Dlclose(audio)
	cf, err := purego.Dlopen("/System/Library/Frameworks/CoreFoundation.framework/CoreFoundation", purego.RTLD_LAZY|purego.RTLD_LOCAL)
	if err != nil {
		return nil, fmt.Errorf("load CoreFoundation: %w", err)
	}
	defer purego.Dlclose(cf)

	var properties coreAudioProperties
	for _, binding := range [...]struct {
		handle uintptr
		name   string
		fn     any
	}{
		{audio, "AudioObjectGetPropertyDataSize", &properties.dataSize},
		{audio, "AudioObjectGetPropertyData", &properties.data},
		{cf, "CFStringGetLength", &properties.length},
		{cf, "CFStringGetBytes", &properties.getBytes},
		{cf, "CFRelease", &properties.release},
	} {
		symbol, err := purego.Dlsym(binding.handle, binding.name)
		if err != nil {
			return nil, fmt.Errorf("resolve %s: %w", binding.name, err)
		}
		purego.RegisterFunc(binding.fn, symbol)
	}

	ids, err := properties.deviceIDs()
	if err != nil {
		return nil, err
	}
	var devices []Device
	inputStreams := audioObjectPropertyAddress{selector: audioPropertyStreams, scope: audioScopeInput}
	for _, id := range ids {
		var size uint32
		if status := properties.dataSize(id, &inputStreams, 0, nil, &size); status != 0 || size == 0 {
			continue
		}
		uid := properties.stringProperty(id, audioPropertyDeviceUID)
		name := properties.stringProperty(id, audioPropertyName)
		// A device can disappear from the HAL while its attributes are read.
		if uid == "" || name == "" {
			continue
		}
		devices = append(devices, Device{UID: uid, Name: name})
	}
	return devices, nil
}

func (p *coreAudioProperties) deviceIDs() ([]uint32, error) {
	address := audioObjectPropertyAddress{selector: audioPropertyDevices, scope: audioScopeGlobal}
	const idSize = uint32(unsafe.Sizeof(uint32(0)))
	// Hotplug can change the array size between looking it up and reading it.
	for range 2 {
		var size uint32
		if status := p.dataSize(audioSystemObject, &address, 0, nil, &size); status != 0 {
			return nil, fmt.Errorf("size CoreAudio device list: OSStatus %d", status)
		}
		if size == 0 {
			return nil, nil
		}
		if size%idSize != 0 {
			return nil, fmt.Errorf("invalid CoreAudio device list size: %d", size)
		}
		ids := make([]uint32, size/idSize)
		used := size
		status := p.data(audioSystemObject, &address, 0, nil, &used, unsafe.Pointer(&ids[0]))
		if status == audioBadPropertySize {
			continue
		}
		if status != 0 {
			return nil, fmt.Errorf("read CoreAudio device list: OSStatus %d", status)
		}
		if used > size || used%idSize != 0 {
			return nil, fmt.Errorf("invalid returned CoreAudio device list size: %d", used)
		}
		return ids[:used/idSize], nil
	}
	return nil, fmt.Errorf("CoreAudio device list changed while being read")
}

func (p *coreAudioProperties) stringProperty(id, selector uint32) string {
	address := audioObjectPropertyAddress{selector: selector, scope: audioScopeGlobal}
	var value uintptr
	const refSize = uint32(unsafe.Sizeof(value))
	size := refSize
	if status := p.data(id, &address, 0, nil, &size, unsafe.Pointer(&value)); status != 0 || size != refSize || value == 0 {
		return ""
	}
	// Both the UID and the name properties transfer a retained CFString.
	defer p.release(value)
	length := p.length(value)
	if length <= 0 {
		return ""
	}
	text := cfRange{length: length}
	var count int
	if converted := p.getBytes(value, text, coreFoundationEncodingUTF8, 0, 0, nil, 0, &count); converted != length || count <= 0 {
		return ""
	}
	buffer := make([]byte, count)
	var used int
	if converted := p.getBytes(value, text, coreFoundationEncodingUTF8, 0, 0, &buffer[0], len(buffer), &used); converted != length || used != count {
		return ""
	}
	return string(buffer)
}
