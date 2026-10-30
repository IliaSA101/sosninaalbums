package app

// Окно Wails: готовность (domReady), минимум и подгонка под экран. Вынесено из app.go, чтобы файл
// не рос; поведение прежнее.

import (
	"context"

	wr "github.com/wailsapp/wails/v2/pkg/runtime"
)

// domReady — окно открыто: размер под экран и проверка библиотеки при запуске (067 Ф3).
func (a *App) domReady(ctx context.Context) {
	defer guard("domReady")
	a.fitScreen(ctx)
	a.goBG(a.placeDemo) // файл примера — раньше проверки при запуске, замок она не теряет
	a.libp.ScheduleCheck()
	a.upd.Schedule()
	a.qaProbe(ctx) // зонд win-qa: только exe набора QA (тег qa) со стендом
}

// Постоянный минимум окна (логические px); на малом экране его уменьшает minSize.
const (
	minWinW = 1200
	minWinH = 640
)

// minSize — минимум окна под экран sw×sh (логические px, 089-D16): Wails умножает минимум
// на масштаб, и при 125 % на 1366×768 постоянный 1200×640 шире и выше экрана — окно
// не сжать до экрана. Неизвестный размер экрана — постоянный минимум.
func minSize(sw, sh int) (w, h int) {
	if sw <= 0 || sh <= 0 {
		return minWinW, minWinH
	}
	return max(1, min(minWinW, sw-40)), max(1, min(minWinH, sh-120))
}

// fitScreen: если обычный (не развёрнутый) размер окна больше экрана —
// уменьшаем его, иначе после «Свернуть в окно» окно выглядит развёрнутым
// и его не за что взять, чтобы перетащить на другой монитор.
// Минимум ставим раньше: WindowSetSize не даёт окну стать меньше минимума.
func (a *App) fitScreen(ctx context.Context) {
	defer guard("fitScreen")
	screens, err := wr.ScreenGetAll(ctx)
	if err != nil {
		return
	}
	for _, s := range screens {
		if !s.IsCurrent {
			continue
		}
		mw, mh := minSize(s.Size.Width, s.Size.Height)
		wr.WindowSetMinSize(ctx, mw, mh)
		w, h := wr.WindowGetSize(ctx)
		maxW, maxH := s.Size.Width-80, s.Size.Height-120
		if w > maxW || h > maxH {
			wasMax := wr.WindowIsMaximised(ctx)
			if wasMax {
				wr.WindowUnmaximise(ctx)
			}
			wr.WindowSetSize(ctx, min(w, maxW), min(h, maxH))
			wr.WindowCenter(ctx)
			if wasMax {
				wr.WindowMaximise(ctx)
			}
		}
	}
}
