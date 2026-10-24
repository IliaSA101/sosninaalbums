package album

import (
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album/cover"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/album/spreads"
	"github.com/IliaSA101/sosninaalbums/internal/album/view"
)

// Разметка типографии (032, 066): файл-подложка обложки и разворотов. Путь — данные проекта
// (правка через change), прочитанное о файле — состояние Editor для вида (слоты guides).

// Слоты разметки в Editor.guides.
const (
	markupCover = iota
	markupSpreads
	markupKinds
)

// SetCoverGuide — файл-разметка обложки (абсолютный путь PSD/JPEG/PNG; проверка путей — как
// у фото при Load: без UNC и устройств). Подложка только для экрана.
func (e *Editor) SetCoverGuide(path string) (model.View, error) {
	if err := salbum.CheckGuide(path); err != nil {
		return e.View(), err
	}
	return e.change(func(p *model.Project) {
		e.guides[markupCover] = guideState{} // тот же путь выбрали заново — файл могли заменить
		cover.SetGuide(p, path)
	}), nil
}

// ClearCoverGuide убирает разметку обложки.
func (e *Editor) ClearCoverGuide() model.View {
	return e.change(func(p *model.Project) { cover.SetGuide(p, "") })
}

// SetSpreadGuide — файл-разметка разворотов (066): один на все развороты (или листы 031),
// один шаг отмены. Проверка пути — как у обложки.
func (e *Editor) SetSpreadGuide(path string) (model.View, error) {
	if err := salbum.CheckGuide(path); err != nil {
		return e.View(), err
	}
	return e.change(func(p *model.Project) {
		e.guides[markupSpreads] = guideState{} // тот же путь выбрали заново — файл могли заменить
		spreads.SetGuide(p, path)
	}), nil
}

// ClearSpreadGuide убирает разметку разворотов; её нет — холостая правка.
func (e *Editor) ClearSpreadGuide() model.View {
	return e.change(func(p *model.Project) { spreads.SetGuide(p, "") })
}

// CoverGuidePath — абсолютный путь файла-разметки обложки ("" — нет); для раздачи /media/cover-guide.
func (e *Editor) CoverGuidePath() (path string) {
	e.read(func(p *model.Project) {
		if p.Cover != nil {
			path = p.Cover.Guide
		}
	})
	return path
}

// SpreadGuidePath — абсолютный путь файла-разметки разворотов ("" — нет); для /media/spread-guide.
func (e *Editor) SpreadGuidePath() (path string) {
	e.read(func(p *model.Project) { path = p.SpreadGuide })
	return path
}

// markupState — прочитанное о разметках для вида. Договор: e.mu взят.
func (e *Editor) markupState(st *view.State) {
	if e.p.Cover != nil {
		st.CoverGuide = e.guideFor(markupCover, e.p.Cover.Guide)
	}
	st.SpreadGuide = e.guideFor(markupSpreads, e.p.SpreadGuide)
}

// guideState — что Editor знает о файле-разметке: сведения, когда читали, идёт ли перечитывание.
type guideState struct {
	view.Guide
	at   time.Time
	busy bool
}

// guideRetry — не чаще этого файл, которого не было (диск не подключён, том не отвечал),
// перечитывается: иначе разметка не появилась бы до смены пути.
const guideRetry = 10 * time.Second

// guideFor — сведения о файле-разметке path слота kind: читаются один раз на путь (смена
// пути — выбор, Undo, Load), а не под каждый вид. Неудача перечитывается в фоне не чаще
// guideRetry: проба мёртвого тома не держит e.mu, найденное покажет следующий вид.
// Договор: e.mu взят.
func (e *Editor) guideFor(kind int, path string) view.Guide {
	g := &e.guides[kind]
	switch {
	case path == "":
	case g.Path != path:
		info, ok := salbum.GuideInfo(path)
		*g = guideState{Guide: view.Guide{Path: path, Info: info, OK: ok}, at: time.Now()}
	case !g.OK && !g.busy && time.Since(g.at) >= guideRetry:
		g.busy, g.at = true, time.Now()
		go e.reprobeGuide(kind, path)
	}
	return g.Guide
}

// reprobeGuide — фоновое перечитывание разметки path без замка; результат кладётся, только
// если путь слота тот же и файла ещё не было (выбор нового пути за это время не затирается).
func (e *Editor) reprobeGuide(kind int, path string) {
	info, ok := salbum.GuideInfo(path)
	e.mu.Lock()
	defer e.mu.Unlock()
	g := &e.guides[kind]
	if g.Path != path || g.OK {
		return
	}
	g.busy, g.at = false, time.Now()
	if ok {
		g.Guide = view.Guide{Path: path, Info: info, OK: true}
	}
}
