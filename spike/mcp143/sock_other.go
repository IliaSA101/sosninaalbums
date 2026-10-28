//go:build !windows

package main

import "os"

func appDataDir() string {
	d, err := os.UserConfigDir()
	if err != nil {
		return os.TempDir()
	}
	return d
}
