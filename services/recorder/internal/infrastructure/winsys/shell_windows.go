package winsys

import (
	"fmt"

	"golang.org/x/sys/windows"
)

// OpenBrowser opens url in the default browser.
func OpenBrowser(url string) error {
	if err := shellOpen(url); err != nil {
		return fmt.Errorf("open the browser: %w", err)
	}
	return nil
}

// shellOpen does what double-clicking target in Explorer would.
func shellOpen(target string) error {
	verb, err := windows.UTF16PtrFromString("open")
	if err != nil {
		return err
	}
	path, err := windows.UTF16PtrFromString(target)
	if err != nil {
		return err
	}
	return windows.ShellExecute(0, verb, path, nil, nil, windows.SW_SHOWNORMAL)
}

// Alert shows an error in a message box: the app has no console, so this is how a failed start is seen.
func Alert(title, text string) {
	caption, err := windows.UTF16PtrFromString(title)
	if err != nil {
		return
	}
	body, err := windows.UTF16PtrFromString(text)
	if err != nil {
		return
	}
	_, _ = windows.MessageBox(0, body, caption, windows.MB_OK|windows.MB_ICONERROR)
}

// DocumentsDir is the user's Documents folder as Windows knows it, which follows a OneDrive-redirected Documents.
func DocumentsDir() (string, error) {
	dir, err := windows.KnownFolderPath(windows.FOLDERID_Documents, 0)
	if err != nil {
		return "", fmt.Errorf("documents folder: %w", err)
	}
	return dir, nil
}
