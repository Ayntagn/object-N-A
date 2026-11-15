package audioinput

import (
	"fmt"
	"runtime"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	eCapture          = 1
	deviceStateActive = 1
	stgmRead          = 0
	vtLPWSTR          = 31
)

var (
	ole32            = windows.NewLazySystemDLL("ole32.dll")
	coCreateInstance = ole32.NewProc("CoCreateInstance")
	propVariantClear = ole32.NewProc("PropVariantClear")

	clsidMMDeviceEnumerator = windows.GUID{Data1: 0xbcde0395, Data2: 0xe52f, Data3: 0x467c,
		Data4: [8]byte{0x8e, 0x3d, 0xc4, 0x57, 0x92, 0x91, 0x69, 0x2e}}
	iidIMMDeviceEnumerator = windows.GUID{Data1: 0xa95664d2, Data2: 0x9614, Data3: 0x4f35,
		Data4: [8]byte{0xa7, 0x46, 0xde, 0x8d, 0xb6, 0x36, 0x17, 0xe6}}
	// PROPERTYKEY has the same GUID and property ID layout as DEVPROPKEY.
	deviceFriendlyName = windows.DEVPROPKEY{
		FmtID: windows.DEVPROPGUID{Data1: 0xa45c254e, Data2: 0xdf1c, Data3: 0x4efd,
			Data4: [8]byte{0x80, 0x20, 0x67, 0xd1, 0x46, 0xa8, 0x50, 0xe0}},
		PID: 14,
	}
)

type unknownVTable struct {
	queryInterface, addRef, release uintptr
}

type mmDeviceEnumerator struct {
	vtable *struct {
		unknownVTable
		enumAudioEndpoints uintptr
	}
}

type mmDeviceCollection struct {
	vtable *struct {
		unknownVTable
		getCount, item uintptr
	}
}

type mmDevice struct {
	vtable *struct {
		unknownVTable
		activate, openPropertyStore, getID uintptr
	}
}

type propertyStore struct {
	vtable *struct {
		unknownVTable
		getCount, getAt, getValue uintptr
	}
}

type propVariant struct {
	kind uint16
	_    [3]uint16
	// The native union occupies 8 bytes on 32-bit and 16 on 64-bit Windows.
	data [2]uintptr
}

// List returns the active audio capture endpoints reported by Windows.
func List() ([]Device, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	initErr := windows.CoInitializeEx(0, windows.COINIT_MULTITHREADED)
	if status, ok := initErr.(windows.Errno); ok && uint32(status) == uint32(windows.RPC_E_CHANGED_MODE) {
		// The caller's thread may already belong to the UI's STA.
		initErr = windows.CoInitializeEx(0, windows.COINIT_APARTMENTTHREADED)
	}
	if initErr != nil && initErr != windows.Errno(windows.S_FALSE) {
		return nil, fmt.Errorf("CoInitializeEx: %w", initErr)
	}
	defer windows.CoUninitialize()

	var enumerator *mmDeviceEnumerator
	status, _, _ := coCreateInstance.Call(uintptr(unsafe.Pointer(&clsidMMDeviceEnumerator)), 0,
		windows.CLSCTX_INPROC_SERVER, uintptr(unsafe.Pointer(&iidIMMDeviceEnumerator)), uintptr(unsafe.Pointer(&enumerator)))
	if err := hresultError("create MMDevice enumerator", status); err != nil {
		return nil, err
	}
	defer releaseCOM(unsafe.Pointer(enumerator))

	var collection *mmDeviceCollection
	status, _, _ = syscall.SyscallN(enumerator.vtable.enumAudioEndpoints, uintptr(unsafe.Pointer(enumerator)),
		eCapture, deviceStateActive, uintptr(unsafe.Pointer(&collection)))
	if err := hresultError("list active capture endpoints", status); err != nil {
		return nil, err
	}
	defer releaseCOM(unsafe.Pointer(collection))

	var count uint32
	status, _, _ = syscall.SyscallN(collection.vtable.getCount, uintptr(unsafe.Pointer(collection)), uintptr(unsafe.Pointer(&count)))
	if err := hresultError("get capture endpoint count", status); err != nil {
		return nil, err
	}
	var devices []Device
	for i := uint32(0); i < count; i++ {
		var endpoint *mmDevice
		status, _, _ = syscall.SyscallN(collection.vtable.item, uintptr(unsafe.Pointer(collection)), uintptr(i), uintptr(unsafe.Pointer(&endpoint)))
		if err := hresultError("get capture endpoint", status); err != nil {
			return nil, err
		}
		device, err := readEndpoint(endpoint)
		if err != nil {
			return nil, err
		}
		devices = append(devices, device)
	}
	return devices, nil
}

func readEndpoint(endpoint *mmDevice) (Device, error) {
	defer releaseCOM(unsafe.Pointer(endpoint))
	var endpointID *uint16
	status, _, _ := syscall.SyscallN(endpoint.vtable.getID, uintptr(unsafe.Pointer(endpoint)), uintptr(unsafe.Pointer(&endpointID)))
	if err := hresultError("get capture endpoint ID", status); err != nil {
		return Device{}, err
	}
	defer windows.CoTaskMemFree(unsafe.Pointer(endpointID))
	uid := windows.UTF16PtrToString(endpointID)
	if uid == "" {
		return Device{}, fmt.Errorf("get capture endpoint ID: empty ID")
	}
	name := endpointName(endpoint)
	if name == "" {
		name = uid
	}
	return Device{UID: uid, Name: name}, nil
}

func endpointName(endpoint *mmDevice) string {
	var store *propertyStore
	status, _, _ := syscall.SyscallN(endpoint.vtable.openPropertyStore, uintptr(unsafe.Pointer(endpoint)), stgmRead, uintptr(unsafe.Pointer(&store)))
	if int32(status) < 0 {
		return ""
	}
	defer releaseCOM(unsafe.Pointer(store))
	var value propVariant
	status, _, _ = syscall.SyscallN(store.vtable.getValue, uintptr(unsafe.Pointer(store)),
		uintptr(unsafe.Pointer(&deviceFriendlyName)), uintptr(unsafe.Pointer(&value)))
	defer func() { propVariantClear.Call(uintptr(unsafe.Pointer(&value))) }()
	if int32(status) < 0 || value.kind != vtLPWSTR {
		return ""
	}
	return windows.UTF16PtrToString(*(**uint16)(unsafe.Pointer(&value.data[0])))
}

func releaseCOM(object unsafe.Pointer) {
	vtable := *(**unknownVTable)(object)
	syscall.SyscallN(vtable.release, uintptr(object))
}

func hresultError(operation string, status uintptr) error {
	if int32(status) < 0 {
		return fmt.Errorf("%s: HRESULT 0x%08X", operation, uint32(status))
	}
	return nil
}
