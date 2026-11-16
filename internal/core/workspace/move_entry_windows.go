package workspace

import (
	"errors"
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

type fileRenameInformation struct {
	ReplaceIfExists byte
	RootDirectory   windows.Handle
	FileNameLength  uint32
	FileName        [windows.MAX_PATH]uint16
}

func renameEntry(fromDir *os.File, from string, toDir *os.File, to string) error {
	name, err := windows.NewNTUnicodeString(from)
	if err != nil {
		return err
	}
	attributes := windows.OBJECT_ATTRIBUTES{
		RootDirectory: windows.Handle(fromDir.Fd()),
		ObjectName:    name,
		Attributes:    windows.OBJ_CASE_INSENSITIVE,
	}
	attributes.Length = uint32(unsafe.Sizeof(attributes))
	var source windows.Handle
	err = windows.NtCreateFile(&source, windows.SYNCHRONIZE|windows.DELETE, &attributes, &windows.IO_STATUS_BLOCK{}, nil, 0,
		windows.FILE_SHARE_DELETE|windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, windows.FILE_OPEN,
		windows.FILE_OPEN_REPARSE_POINT|windows.FILE_OPEN_FOR_BACKUP_INTENT|windows.FILE_SYNCHRONOUS_IO_NONALERT, 0, 0)
	if err != nil {
		return renameWindowsError(err)
	}
	defer windows.CloseHandle(source)
	target, err := windows.UTF16FromString(to)
	if err != nil {
		return err
	}
	var info fileRenameInformation
	// A simple name with no RootDirectory renames within the source directory.
	if fromDir != toDir {
		info.RootDirectory = windows.Handle(toDir.Fd())
	}
	if len(target) > len(info.FileName) {
		return windows.ERROR_FILENAME_EXCED_RANGE
	}
	info.FileNameLength = uint32((len(target) - 1) * 2)
	copy(info.FileName[:], target)
	const fileRenameInformationClass = 10
	return renameWindowsError(windows.NtSetInformationFile(source, &windows.IO_STATUS_BLOCK{},
		(*byte)(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info)), fileRenameInformationClass))
}

func renameWindowsError(err error) error {
	if status, ok := err.(windows.NTStatus); ok {
		return status.Errno()
	}
	return err
}

func unsupportedLifecycleCommit(err error) bool {
	return errors.Is(err, errors.ErrUnsupported) || errors.Is(err, windows.ERROR_INVALID_FUNCTION)
}
