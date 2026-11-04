//go:build gc && !noasm && !appengine

package testkit

// SIMDPath — x/image/vector считает покрытие ассемблером SSE4.1 (acc_amd64.s), а не чистым Go: те же условия сборки,
// что у acc_amd64.go. Пиксельные эталоны рисунка (decor/gen) сняты на пути SIMD; чистый Go даёт тот же рисунок с
// отличием на 1 уровень в отдельных байтах (класс B, 162).
const SIMDPath = true
