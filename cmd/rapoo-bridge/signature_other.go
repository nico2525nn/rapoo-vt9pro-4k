//go:build !windows

package main

// verifyUpdaterSignature is only meaningful on Windows.
func verifyUpdaterSignature(string) error {
	return nil
}
