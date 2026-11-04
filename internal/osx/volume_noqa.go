//go:build !qa

package osx

import "io/fs"

// qaStat — шов задержки пробы тома есть только в exe набора QA (volume_qa.go); в выпуске и в dev
// проба идёт напрямую.
func qaStat(stat func(string) (fs.FileInfo, error)) func(string) (fs.FileInfo, error) { return stat }
