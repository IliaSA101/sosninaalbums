//go:build !race

package testkit

// RaceSlowdown — запас для порогов времени в тестах: без детектора гонок не нужен.
const RaceSlowdown = 1
