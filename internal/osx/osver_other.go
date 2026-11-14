//go:build !darwin && !windows

package osx

// osVersion — для Linux и прочих правила minOS нет.
func osVersion() (string, bool) { return "", false }
