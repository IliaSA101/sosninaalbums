//go:build !race

package testkit

// RaceSlowdown — запас для порогов времени в тестах: без детектора гонок не нужен.
const RaceSlowdown = 1

// Race — прогон с детектором гонок (здесь без него).
const Race = false
