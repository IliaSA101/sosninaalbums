package osx

import "os/exec"

// Start запускает exe с аргументами отдельным процессом и не ждёт его: «Открыть в 0.X»
// (спека 112) передаёт новой версии файл и сама выходит. Оболочки нет — аргументы идут как
// есть. Процесс отпускается (Release), поэтому переживает выход родителя.
func Start(exe string, args ...string) error {
	cmd := exec.Command(exe, args...) //nolint:gosec // exe — найденная установка (путь из реестра, проверен на существование), оболочки нет
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}
