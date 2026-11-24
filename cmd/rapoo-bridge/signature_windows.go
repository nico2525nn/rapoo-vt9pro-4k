//go:build windows

package main

import (
	"errors"
	"fmt"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// verifyUpdaterSignature rejects a tampered Authenticode signature. Unsigned
// official tools are still allowed here; the Windows UI asks for explicit
// confirmation and checks the signer subject when a certificate is present.
func verifyUpdaterSignature(path string) error {
	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	fileInfo := windows.WinTrustFileInfo{
		Size:     uint32(unsafe.Sizeof(windows.WinTrustFileInfo{})),
		FilePath: pathPtr,
	}
	data := &windows.WinTrustData{
		Size:                            uint32(unsafe.Sizeof(windows.WinTrustData{})),
		UIChoice:                        windows.WTD_UI_NONE,
		RevocationChecks:                windows.WTD_REVOKE_NONE,
		UnionChoice:                     windows.WTD_CHOICE_FILE,
		StateAction:                     windows.WTD_STATEACTION_VERIFY,
		FileOrCatalogOrBlobOrSgnrOrCert: unsafe.Pointer(&fileInfo),
	}
	verifyErr := windows.WinVerifyTrustEx(
		windows.InvalidHWND,
		&windows.WINTRUST_ACTION_GENERIC_VERIFY_V2,
		data)
	data.StateAction = windows.WTD_STATEACTION_CLOSE
	_ = windows.WinVerifyTrustEx(
		windows.InvalidHWND,
		&windows.WINTRUST_ACTION_GENERIC_VERIFY_V2,
		data)

	if verifyErr == nil ||
		errors.Is(verifyErr, syscall.Errno(uintptr(windows.TRUST_E_NOSIGNATURE))) ||
		errors.Is(verifyErr, syscall.Errno(uintptr(windows.TRUST_E_SUBJECT_FORM_UNKNOWN))) {
		return nil
	}
	return fmt.Errorf("updater Authenticode verification failed: %w", verifyErr)
}
