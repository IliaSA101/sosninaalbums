//go:build !unix

package layout

// cpuTimeKnown — на этой ОС процессорное время недоступно: TestWarmCropCost пропускается.
const cpuTimeKnown = false

func cpuSeconds() float64 { return 0 }
