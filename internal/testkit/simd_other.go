//go:build !amd64 || !gc || noasm || appengine

package testkit

// SIMDPath — см. simd_amd64.go: здесь x/image/vector считает чистым Go.
const SIMDPath = false
