package winsys

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"syscall"
	"unsafe"

	"github.com/go-ole/go-ole"
	"golang.org/x/sys/windows"
)

const (
	clsidFileOpenDialog = "{DC1C5A9C-E88A-4DDE-A5A1-60F82A20AEF7}"
	iidFileOpenDialog   = "{D57C7288-D4AD-4768-BE02-9D969532D960}"
	iidShellItem        = "{43826D1E-E718-42EE-BC55-A1E261C37BFE}"
	fosPickFolders      = 0x20
	fosForceFileSystem  = 0x40
	sigdnFileSysPath    = 0x80058000
	// hresultCancelled is HRESULT_FROM_WIN32(ERROR_CANCELLED): the owner closed the dialog without choosing.
	hresultCancelled = 0x800704C7
	// lower32 keeps the HRESULT a COM method returns in the low half of the register.
	lower32 = 0xFFFFFFFF
	// sFalse is S_FALSE: COM was already initialised on this thread in the same model.
	sFalse         = 1
	wsExTopmost    = 0x00000008
	wsExToolWindow = 0x00000080
	wsPopup        = 0x80000000
)

var errNoFolder = errors.New("folder dialog: no folder came back")

// fileDialogVtbl is IFileOpenDialog's method table, up to the last method used.
type fileDialogVtbl struct {
	ole.IUnknownVtbl

	Show                uintptr
	SetFileTypes        uintptr
	SetFileTypeIndex    uintptr
	GetFileTypeIndex    uintptr
	Advise              uintptr
	Unadvise            uintptr
	SetOptions          uintptr
	GetOptions          uintptr
	SetDefaultFolder    uintptr
	SetFolder           uintptr
	GetFolder           uintptr
	GetCurrentSelection uintptr
	SetFileName         uintptr
	GetFileName         uintptr
	SetTitle            uintptr
	SetOkButtonLabel    uintptr
	SetFileNameLabel    uintptr
	GetResult           uintptr
}

type shellItemVtbl struct {
	ole.IUnknownVtbl

	BindToHandler  uintptr
	GetParent      uintptr
	GetDisplayName uintptr
}

// FolderPicker shows Windows' own folder dialog, which allows typing or pasting a path.
type FolderPicker struct{}

// Pick blocks until the owner chooses a folder or cancels; cancelling is not an error.
func (FolderPicker) Pick(_ context.Context, start string) (string, bool, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED); err != nil && !isSFalse(err) {
		return "", false, fmt.Errorf("folder dialog: initialise COM: %w", err)
	}
	defer ole.CoUninitialize()

	dialog, err := ole.CreateInstance(ole.NewGUID(clsidFileOpenDialog), ole.NewGUID(iidFileOpenDialog))
	if err != nil {
		return "", false, fmt.Errorf("folder dialog: %w", err)
	}
	defer dialog.Release()
	vt := (*fileDialogVtbl)(unsafe.Pointer(dialog.RawVTable))
	self := uintptr(unsafe.Pointer(dialog))

	var options uint32
	if r, _, _ := syscall.SyscallN(vt.GetOptions, self, uintptr(unsafe.Pointer(&options))); r&lower32 != 0 {
		return "", false, fmt.Errorf("folder dialog: options: %w", ole.NewError(r&lower32))
	}
	opts := uintptr(options) | fosPickFolders | fosForceFileSystem
	if r, _, _ := syscall.SyscallN(vt.SetOptions, self, opts); r&lower32 != 0 {
		return "", false, fmt.Errorf("folder dialog: options: %w", ole.NewError(r&lower32))
	}
	setStartFolder(vt, self, start)

	owner := topmostOwner()
	defer destroyWindow(owner)
	r, _, _ := syscall.SyscallN(vt.Show, self, owner)
	switch r & lower32 {
	case 0:
	case hresultCancelled:
		return "", false, nil
	default:
		return "", false, fmt.Errorf("folder dialog: %w", ole.NewError(r&lower32))
	}
	return result(vt, self)
}

func result(vt *fileDialogVtbl, self uintptr) (string, bool, error) {
	var item *ole.IUnknown
	if r, _, _ := syscall.SyscallN(vt.GetResult, self, uintptr(unsafe.Pointer(&item))); r&lower32 != 0 || item == nil {
		return "", false, errNoFolder
	}
	defer item.Release()
	ivt := (*shellItemVtbl)(unsafe.Pointer(item.RawVTable))
	var name *uint16
	r, _, _ := syscall.SyscallN(
		ivt.GetDisplayName, uintptr(unsafe.Pointer(item)), sigdnFileSysPath, uintptr(unsafe.Pointer(&name)),
	)
	if r&lower32 != 0 || name == nil {
		return "", false, errNoFolder
	}
	defer ole.CoTaskMemFree(uintptr(unsafe.Pointer(name)))
	return windows.UTF16PtrToString(name), true, nil
}

// setStartFolder opens the dialog where the current folder is; a folder that no longer exists is simply skipped.
func setStartFolder(vt *fileDialogVtbl, self uintptr, start string) {
	if start == "" {
		return
	}
	path, err := windows.UTF16PtrFromString(start)
	if err != nil {
		return
	}
	var item *ole.IUnknown
	create := windows.NewLazySystemDLL("shell32.dll").NewProc("SHCreateItemFromParsingName")
	r, _, _ := create.Call(
		uintptr(
			unsafe.Pointer(path),
		),
		0,
		uintptr(unsafe.Pointer(ole.NewGUID(iidShellItem))),
		uintptr(unsafe.Pointer(&item)),
	)
	if r&lower32 != 0 || item == nil {
		return
	}
	defer item.Release()
	_, _, _ = syscall.SyscallN(vt.SetFolder, self, uintptr(unsafe.Pointer(item)))
}

// topmostOwner is a hidden window that keeps the dialog above the browser: Windows will not bring a background
// process's window to the front, but a window owned by a topmost one stays on top.
func topmostOwner() uintptr {
	class, err := windows.UTF16PtrFromString("STATIC")
	if err != nil {
		return 0
	}
	create := windows.NewLazySystemDLL("user32.dll").NewProc("CreateWindowExW")
	hwnd, _, _ := create.Call(wsExTopmost|wsExToolWindow, uintptr(unsafe.Pointer(class)), 0, wsPopup,
		0, 0, 0, 0, 0, 0, 0, 0)
	return hwnd
}

func destroyWindow(hwnd uintptr) {
	if hwnd == 0 {
		return
	}
	destroy := windows.NewLazySystemDLL("user32.dll").NewProc("DestroyWindow")
	_, _, _ = destroy.Call(hwnd)
}

func isSFalse(err error) bool {
	var oleErr *ole.OleError
	return errors.As(err, &oleErr) && oleErr.Code() == sFalse
}
