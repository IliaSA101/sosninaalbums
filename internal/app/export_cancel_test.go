package app

import (
	"slices"
	"strings"
	"testing"
)

// Остановка экспорта (073 §4): общие помощники — exportEnv в export_test.go.

// doneArgs — итог последнего export:done: err, note, memo, stopped и флаг экспорта на момент события.
func (e *exportEnv) doneArgs(t *testing.T) (msg, note string, memo []string, stopped, busy bool) {
	t.Helper()
	if len(e.last) != 6 {
		t.Fatalf("export:done: %d аргументов, ждали dir, err, note, memo, stopped (+флаг): %v", len(e.last), e.last)
	}
	memo, _ = e.last[3].([]string)
	return e.last[1].(string), e.last[2].(string), memo, e.last[4].(bool), e.last[5].(bool)
}

// cancelAt — «Остановить» сразу после готового разворота k.
func (e *exportEnv) cancelAt(k int) {
	e.onStep = func(d int) {
		if d == k {
			e.a.CancelExport()
		}
	}
}

// exportIdle — экспорта нет: флаг и отмена сняты.
func (e *exportEnv) exportIdle(t *testing.T) {
	t.Helper()
	e.a.mu.Lock()
	defer e.a.mu.Unlock()
	if e.a.exporting || e.a.exportCancel != nil {
		t.Fatalf("экспорт не снят: идёт %v, отмена %v", e.a.exporting, e.a.exportCancel != nil)
	}
}

// Остановка посреди выгрузки поверх прежней: готовый разворот переписан, остальные прежние на
// месте (не удалены и не переписаны), итог «остановлен», флаг снят до события; повтор пишет всё.
func TestExportCancelMidway(t *testing.T) {
	e := newExportEnv(t, 3)
	e.export(t, ExportOptions{})
	e.oldify(t)
	e.cancelAt(1)
	e.export(t, ExportOptions{})
	msg, note, memo, stopped, busy := e.doneArgs(t)
	want := "Экспорт остановлен: готово 1 из 3. Уже выгруженные файлы остались в папке."
	if msg != "" || note != want || memo != nil || !stopped || busy {
		t.Fatalf("итог: %q %q %v остановлен %v, флаг при событии %v", msg, note, memo, stopped, busy)
	}
	if !e.rewritten(t, "01.jpg") || e.rewritten(t, "02.jpg") || e.rewritten(t, "03.jpg") {
		t.Fatal("после остановки на 1-м переписан не только 01.jpg")
	}
	e.exportIdle(t)

	e.onStep = nil
	e.export(t, ExportOptions{})
	if msg, note, _, stopped, busy := e.doneArgs(t); msg != "" || stopped || busy || strings.Contains(note, "остановлен") {
		t.Fatalf("повтор: %q %q остановлен %v, флаг %v", msg, note, stopped, busy)
	}
	for _, n := range []string{"01.jpg", "02.jpg", "03.jpg"} {
		if !e.rewritten(t, n) {
			t.Errorf("повтор не переписал %s", n)
		}
	}
	e.exportIdle(t)
}

// Частичные файлы: в пустой папке остаются только готовые; при смене вида (целиком → по
// страницам) прежние целые не удаляются — их удалила бы только полная выгрузка.
func TestExportCancelPartial(t *testing.T) {
	e := newExportEnv(t, 4)
	e.cancelAt(2)
	e.export(t, ExportOptions{})
	if got, want := e.names(t), []string{"01.jpg", "02.jpg"}; !slices.Equal(got, want) {
		t.Fatalf("в новой папке %v, ждали %v", got, want)
	}
	if _, note, _, stopped, _ := e.doneArgs(t); !stopped || !strings.HasPrefix(note, "Экспорт остановлен: готово 2 из 4.") {
		t.Fatalf("итог: %q остановлен %v", note, stopped)
	}

	e.onStep = nil
	e.export(t, ExportOptions{}) // все 4 целиком
	e.cancelAt(1)
	e.export(t, ExportOptions{Pages: true})
	want := []string{"01-1.jpg", "01-2.jpg", "01.jpg", "02.jpg", "03.jpg", "04.jpg"}
	if got := e.names(t); !slices.Equal(got, want) {
		t.Fatalf("после остановки по страницам %v, ждали %v", got, want)
	}
	e.exportIdle(t)
}

// Выход из программы посреди экспорта — тоже «остановлен», а не ошибка.
func TestExportStopOnExit(t *testing.T) {
	e := newExportEnv(t, 3)
	e.onStep = func(d int) {
		if d == 2 {
			e.a.stopCancel()
		}
	}
	e.export(t, ExportOptions{})
	msg, note, _, stopped, busy := e.doneArgs(t)
	if msg != "" || !stopped || busy || !strings.HasPrefix(note, "Экспорт остановлен: готово 2 из 3.") {
		t.Fatalf("итог: %q %q остановлен %v, флаг %v", msg, note, stopped, busy)
	}
	if got := e.names(t); !slices.Equal(got, []string{"01.jpg", "02.jpg"}) {
		t.Fatalf("файлы %v", got)
	}
}

// «Остановить» без экспорта — холостое: до первого и после законченного ничего не меняет,
// следующий экспорт идёт целиком.
func TestCancelExportIdle(t *testing.T) {
	e := newExportEnv(t, 2)
	e.a.CancelExport()
	e.exportIdle(t)
	e.export(t, ExportOptions{})
	e.a.CancelExport()
	e.exportIdle(t)
	e.export(t, ExportOptions{})
	if msg, _, _, stopped, _ := e.doneArgs(t); msg != "" || stopped {
		t.Fatalf("после холостой отмены: %q остановлен %v", msg, stopped)
	}
	if got := e.names(t); !slices.Equal(got, []string{"01.jpg", "02.jpg"}) {
		t.Fatalf("файлы %v", got)
	}
}
