//go:build windows || darwin

package paths

// foldCase — NTFS и APFS (по умолчанию) не различают регистр: ключи путей — в нижнем регистре.
const foldCase = true
