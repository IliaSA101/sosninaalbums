package exporter

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/app/appkit"
	"github.com/IliaSA101/sosninaalbums/internal/export"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
	"github.com/IliaSA101/sosninaalbums/internal/render"
)

// exportJob — проверенная выгрузка: развороты, план, папка и что сказать по итогу.
type exportJob struct {
	spreads []render.Spread
	pl      export.Plan
	opt     render.Options
	dir     string
	stale   []string       // прежние файлы: удаляются только после записи всех новых
	conv    map[string]int // пересчитанные в sRGB фото по именам профилей — для строки о цвете
	memo    []string       // памятка типографии; nil — обычный тост
}

// runExport — фон экспорта и итог export:done (dir, err, note, memo, stopped). Остановка
// (CancelExport или выход из программы) прерывает текущий разворот (ctx доходит до декодов и ожидания
// бюджета памяти, 080-8), недописанного файла нет: готовые файлы остаются,
// прежние не удаляются.
func (s *Service) runExport(ctx context.Context, cancel context.CancelFunc, j exportJob) {
	defer appkit.Guard("экспорт")
	// Флаг снимается один раз — до export:done; здесь только при панике до него: после события
	// флаг может уже принадлежать новому экспорту.
	ended := false
	defer func() {
		if !ended {
			s.endAs(StateFailed, errAborted)
		}
	}()
	defer cancel()
	restoreMem := s.h.LiftMemory()
	defer restoreMem() // при панике; обычно — до export:done ниже
	start := time.Now()
	done, total := 0, planTotal(j.pl)
	err := export.RunPlan(ctx, j.spreads, j.pl, j.opt, j.dir, j.stale, func(d, t int) {
		done, total = d, t
		s.progress(d, t)
		s.h.Emit("export:progress", d, t)
	})
	msg, note, memo, stopped := "", "", []string(nil), false
	state, agentErr := StateDone, "" // для агента: ошибка записи без пути (в msg он есть)
	switch {
	case errors.Is(err, context.Canceled):
		state = StateStopped
		note, stopped = fmt.Sprintf("Экспорт остановлен: готово %d из %d. Уже выгруженные файлы остались в папке.", done, total), true
		log.Printf("экспорт: остановлен, готово %d из %d", done, total)
	case err != nil:
		state, agentErr = StateFailed, errWrite
		msg = err.Error()
		log.Printf("экспорт: %v", paths.NoPath(err))
	default:
		note, memo = export.ColorNote(j.conv), j.memo
		log.Printf("экспорт: %d разворотов за %v", len(j.pl.Jobs), time.Since(start).Round(time.Second))
		album.LogMem("экспорт")
		if note != "" {
			log.Print(note)
		}
	}
	// Предел и флаг — до события: окно по export:done снова даёт «Экспорт», и повтор сразу не должен
	// получить «экспорт уже идёт» или застать снятый предел.
	restoreMem()
	s.progress(done, total)
	s.endAs(state, agentErr)
	ended = true
	s.h.Emit("export:done", j.dir, msg, note, memo, stopped)
}

// Тексты ошибок для агента: без пути (ExportState.Error).
const (
	errWrite   = "не удалось записать файлы выгрузки"
	errAborted = "выгрузка прервана"
)

// planTotal — единиц хода плана: задания разворотов и обложка (как считает export.RunPlan).
func planTotal(pl export.Plan) int {
	n := len(pl.Jobs)
	if pl.Cover != nil {
		n++
	}
	return n
}

// endAs снимает флаг экспорта и его отмену и закрывает ход для агента: состояние state, текст
// ошибки errText (без пути). Ход не «идёт» — ничего не меняется.
func (s *Service) endAs(state, errText string) {
	s.mu.Lock()
	s.busy, s.cancel = false, nil
	if s.last.State == StateRunning {
		s.last.State, s.last.Error = state, errText
	}
	s.mu.Unlock()
}

// Cancel — «Остановить»: текущий разворот прерывается (не дописывается), следующие не пишутся, прежние
// файлы папки не удаляются. Экспорта нет — ничего.
func (s *Service) Cancel() {
	s.mu.Lock()
	cancel := s.cancel
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}
