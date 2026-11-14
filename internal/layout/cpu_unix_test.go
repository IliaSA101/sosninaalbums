//go:build unix

package layout

import "syscall"

// cpuTimeKnown — процессорное время процесса доступно (TestWarmCropCost).
const cpuTimeKnown = true

// cpuSeconds — процессорное время процесса (пользователь + система), с: на занятой машине
// стенное время врёт, а прогрев считается в нескольких потоках.
func cpuSeconds() float64 {
	var u syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &u); err != nil {
		return 0
	}
	return float64(u.Utime.Sec+u.Stime.Sec) + float64(u.Utime.Usec+u.Stime.Usec)/1e6
}
