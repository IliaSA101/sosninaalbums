//go:build race

package testkit

// RaceSlowdown — запас для порогов времени в тестах: детектор гонок замедляет код в разы.
const RaceSlowdown = 5
