package testkit

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"strings"
	"testing"
)

// NoFMAFlag — ключ сборки, выключающий слияние x*y±z (FMA) во всех пакетах. Go разрешает сливать, и arm64 сливает
// сложение и вычитание, amd64 с GOAMD64=v3 — сложение; эталоны сняты без слияния (Windows и Linux — v1), поэтому на Mac
// их считает только сборка с ключом. «q» — тихий режим: без него компилятор печатает строку на каждое место (162).
const NoFMAFlag = "all=-d=fmahash=qn"

// noFMAHint — как собрать; один текст на оба провала стража.
const noFMAHint = "включено слияние FMA: соберите с GOFLAGS=-gcflags=" + NoFMAFlag + " (162)"

// RequireNoFMA — страж эталонов, снятых без слияния FMA: провал (не пропуск), если сборка его допускает. Пропуск
// оставил бы зелёным шаг с забытым GOFLAGS и пустую блокировку (0.36-162-13). Две проверки:
//  1. канарейка — слияние на самом деле происходит (ловит и ключ, который не дошёл до пакета);
//  2. сборочная информация — там, где слияние возможно (не amd64 или GOAMD64 ≥ v3), в -gcflags должен быть ключ с
//     all=: -gcflags в командной строке перекрывает GOFLAGS, а ключ без all= не трогает math и зависимости.
func RequireNoFMA(t testing.TB) {
	t.Helper()
	if err := noFMAError(); err != nil {
		t.Fatal(err)
	}
}

// noFMAError — причина провала стража или nil.
func noFMAError() error {
	var settings []debug.BuildSetting
	if bi, ok := debug.ReadBuildInfo(); ok {
		settings = bi.Settings
	}
	return noFMACheck(settings, fmaFused(), runtime.GOARCH)
}

// noFMACheck — чистое решение стража: настройки сборки, итог канарейки и архитектура цели на входе (тест проверяет
// и чужие архитектуры).
func noFMACheck(settings []debug.BuildSetting, fused bool, goarch string) error {
	if fused {
		return fmt.Errorf("%s: канарейка слита", noFMAHint)
	}
	var gcflags, amd64 string
	for _, s := range settings {
		switch s.Key {
		case "-gcflags":
			gcflags = s.Value
		case "GOAMD64":
			amd64 = s.Value
		}
	}
	if !fmaPossible(goarch, amd64) {
		return nil
	}
	if !hasField(gcflags, NoFMAFlag) {
		return fmt.Errorf("%s: в сборочной информации -gcflags=%q", noFMAHint, gcflags)
	}
	return nil
}

// fmaPossible — компилятор этой цели вправе сливать: не amd64 (arm64 и прочие) или GOAMD64 v3 и выше.
func fmaPossible(goarch, goamd64 string) bool {
	return goarch != "amd64" || goamd64 >= "v3"
}

// hasField — flag есть среди слов строки gcflags (паттерн `all=` — часть слова, а не любого подстроки).
func hasField(gcflags, flag string) bool {
	for _, f := range strings.Fields(gcflags) {
		if f == flag {
			return true
		}
	}
	return false
}

// fmaFused — слился ли a*b±c в канарейках. a = 1+2⁻³⁰, b = 1−2⁻³⁰: a*b = 1−2⁻⁶⁰ округляется до 1, без слияния обе
// функции дают 0; слитое сложение даёт −2⁻⁶⁰, слитое вычитание +2⁻⁶⁰. Умножение и сложение — внутри
// //go:noinline-функций: без вставки в место вызова компилятор не свернёт их константами.
func fmaFused() bool {
	a, b := 1+0x1p-30, 1-0x1p-30
	return fmaAdd(a, b, -1) != 0 || fmaSub(a, b, 1) != 0
}

//go:noinline
func fmaAdd(a, b, c float64) float64 { return a*b + c }

//go:noinline
func fmaSub(a, b, d float64) float64 { return d - a*b }
