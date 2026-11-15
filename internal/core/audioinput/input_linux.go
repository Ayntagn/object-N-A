package audioinput

import (
	"fmt"
	"strings"
	"syscall"
	"unsafe"

	"github.com/ebitengine/purego"
)

// List returns the PCM inputs advertised by ALSA.
func List() ([]Device, error) {
	handle, err := purego.Dlopen("libasound.so.2", purego.RTLD_NOW|purego.RTLD_LOCAL)
	if err != nil {
		return nil, fmt.Errorf("load ALSA: %w", err)
	}
	defer purego.Dlclose(handle)

	var listHints func(int32, string, **unsafe.Pointer) int32
	var getHint func(unsafe.Pointer, string) unsafe.Pointer
	var freeHints func(*unsafe.Pointer) int32
	var freeString func(unsafe.Pointer)
	for _, binding := range [...]struct {
		name string
		fn   any
	}{
		{"snd_device_name_hint", &listHints},
		{"snd_device_name_get_hint", &getHint},
		{"snd_device_name_free_hint", &freeHints},
		{"free", &freeString},
	} {
		symbol, err := purego.Dlsym(handle, binding.name)
		if err != nil {
			return nil, fmt.Errorf("resolve ALSA symbol %s: %w", binding.name, err)
		}
		purego.RegisterFunc(binding.fn, symbol)
	}

	var hints *unsafe.Pointer
	if status := listHints(-1, "pcm", &hints); status < 0 {
		return nil, fmt.Errorf("list ALSA PCM hints: %w", syscall.Errno(-status))
	}
	if hints == nil {
		return nil, nil
	}
	defer freeHints(hints)

	value := func(hint unsafe.Pointer, field string) string {
		ptr := getHint(hint, field)
		if ptr == nil {
			return ""
		}
		defer freeString(ptr)
		length := 0
		for *(*byte)(unsafe.Add(ptr, length)) != 0 {
			length++
		}
		return string(unsafe.Slice((*byte)(ptr), length))
	}

	var devices []Device
	seen := make(map[string]struct{})
	// The hint array ends with a null pointer.
	for entry := hints; *entry != nil; entry = (*unsafe.Pointer)(unsafe.Add(unsafe.Pointer(entry), unsafe.Sizeof(*entry))) {
		hint := *entry
		uid := value(hint, "NAME")
		switch uid {
		case "", "default", "null":
			continue
		}
		if _, duplicate := seen[uid]; duplicate || value(hint, "IOID") == "Output" {
			continue
		}
		name, _, _ := strings.Cut(strings.TrimSpace(value(hint, "DESC")), "\n")
		name = strings.TrimSpace(name)
		if name == "" {
			name = uid
		}
		seen[uid] = struct{}{}
		devices = append(devices, Device{UID: uid, Name: name})
	}
	return devices, nil
}
