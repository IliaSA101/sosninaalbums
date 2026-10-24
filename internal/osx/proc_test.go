package osx

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// TestHelperChild — не тест: тело дочернего процесса, запускаемого тестами ниже (роль задаёт
// переменная окружения). Без неё сразу выходит и ничего не проверяет.
func TestHelperChild(t *testing.T) {
	mode := os.Getenv("SA_OSX_CHILD")
	if mode == "" {
		return
	}
	if mode == "sleep" {
		d, _ := time.ParseDuration(os.Getenv("SA_OSX_CHILD_ARG"))
		time.Sleep(d)
		os.Exit(0)
	}
	// "write": файл-метка, что процесс запущен
	if err := os.WriteFile(os.Getenv("SA_OSX_CHILD_ARG"), []byte("ok"), 0o600); err != nil {
		os.Exit(2)
	}
	os.Exit(0)
}

// child — дочерний процесс-«спальщик»; Wait в фоне подбирает зомби (kill(pid,0) видит его живым).
func child(t *testing.T, sleep string) (cmd *exec.Cmd, done chan struct{}) {
	t.Helper()
	cmd = exec.Command(os.Args[0], "-test.run=^TestHelperChild$")
	cmd.Env = append(os.Environ(), "SA_OSX_CHILD=sleep", "SA_OSX_CHILD_ARG="+sleep)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done = make(chan struct{})
	go func() { _ = cmd.Wait(); close(done) }()
	t.Cleanup(func() { _ = cmd.Process.Kill(); <-done })
	return cmd, done
}

// WaitExit: короткий процесс — true до предела; долгий — false по пределу, true после убийства;
// вышедший, pid ≤ 0 — сразу true.
func TestWaitExit(t *testing.T) {
	cmd, done := child(t, "300ms")
	pid := cmd.Process.Pid
	if !WaitExit(pid, 30*time.Second) {
		t.Fatal("WaitExit не дождался выхода короткого процесса")
	}
	<-done
	if !WaitExit(pid, time.Second) { // уже вышел и подобран
		t.Error("WaitExit для вышедшего процесса должен вернуть true")
	}

	cmd, done = child(t, "1m")
	pid = cmd.Process.Pid
	start := time.Now()
	if WaitExit(pid, 250*time.Millisecond) {
		t.Fatal("WaitExit вернул true для живого процесса")
	}
	if d := time.Since(start); d < 200*time.Millisecond {
		t.Errorf("WaitExit вернулся раньше предела: %v", d)
	}
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	<-done
	if !WaitExit(pid, 30*time.Second) {
		t.Error("WaitExit не увидел выход убитого процесса")
	}

	for _, p := range []int{0, -1} {
		if !WaitExit(p, time.Second) {
			t.Errorf("WaitExit(%d) должен вернуть true", p)
		}
	}
}

// Start: процесс запускается с аргументами и без ожидания; нет exe — ошибка.
func TestStart(t *testing.T) {
	mark := filepath.Join(t.TempDir(), "mark")
	t.Setenv("SA_OSX_CHILD", "write")
	t.Setenv("SA_OSX_CHILD_ARG", mark)
	if err := Start(os.Args[0], "-test.run=^TestHelperChild$"); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(30 * time.Second)
	for {
		if _, err := os.Stat(mark); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("запущенный процесс не отработал")
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err := Start(filepath.Join(t.TempDir(), "нет-такого.exe")); err == nil {
		t.Error("Start несуществующего exe без ошибки")
	}
}
