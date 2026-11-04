package edit

import (
	"math"
	"path/filepath"
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// stickerOf — копия стикера id из снимка проекта (nil — нет).
func stickerOf(m *Methods, id string) *model.Sticker {
	p, _ := m.editor().Snapshot()
	s, i := p.FindSticker(id)
	if s == nil {
		return nil
	}
	return s.Stickers[i].Clone()
}

// Рамка, зеркало, прозрачность и цвета стикера через методы App: серия merge — один шаг,
// холостая правка — без шага и без смены ревизии, отмена возвращает прежнее; два стикера
// на разных листах не мешают друг другу; правки переживают запись и чтение файла.
func TestStickerSurface(t *testing.T) {
	m, views := spreadsMethods(t, []string{"a"}, []string{"b"})
	ed := m.editor()
	_, st1, err := ed.AddSticker("s1", 0, model.StickerSrc{Art: &model.Art{Kind: "star"}})
	if err != nil {
		t.Fatal(err)
	}
	_, st2, err := ed.AddSticker("s2", 1, model.StickerSrc{Art: &model.Art{Kind: "heart"}})
	if err != nil {
		t.Fatal(err)
	}
	other := stickerOf(m, st2)
	start := stickerOf(m, st1)
	rev := ed.View().Rev

	// Серия жеста: два шага рамки с одним merge — один шаг отмены.
	m.SetStickerBox("s1", st1, start.X+5, start.Y+3, start.W, start.H, 0, "g1")
	v := m.SetStickerBox("s1", st1, start.X+10, start.Y+6, start.W, start.H, 15, "g1")
	if got := stickerOf(m, st1); math.Abs(got.X-(start.X+10)) > 0.011 || got.Rot != 15 {
		t.Fatalf("рамка: %+v", got)
	}
	if v.Rev == rev || len(views()) != 2 {
		t.Fatalf("рамка: rev %d→%d, видов разослано %d", rev, v.Rev, len(views()))
	}
	// Холостая: те же числа и не конечные — ревизия та же.
	got := stickerOf(m, st1)
	if v2 := m.SetStickerBox("s1", st1, got.X, got.Y, got.W, got.H, got.Rot, "g2"); v2.Rev != v.Rev {
		t.Errorf("та же рамка — правка: rev %d→%d", v.Rev, v2.Rev)
	}
	if v2 := m.SetStickerBox("s1", st1, math.NaN(), 0, 10, 10, 0, ""); v2.Rev != v.Rev {
		t.Errorf("NaN в рамке — правка: rev %d→%d", v.Rev, v2.Rev)
	}
	m.undo()
	if got := stickerOf(m, st1); got.X != start.X || got.Y != start.Y || got.Rot != start.Rot {
		t.Fatalf("серия рамки не одним шагом: %+v, было %+v", got, start)
	}
	m.redo()

	// Зеркало: шаг; ось не h/v — холостая.
	v = m.FlipSticker("s1", st1, "h")
	if !stickerOf(m, st1).FlipH {
		t.Fatal("зеркало h не встало")
	}
	if v2 := m.FlipSticker("s1", st1, "x"); v2.Rev != v.Rev || stickerOf(m, st1).FlipV {
		t.Errorf("ось x — правка: rev %d→%d", v.Rev, v2.Rev)
	}
	v = m.FlipSticker("s1", st1, "v")

	// Прозрачность: серия ползунка — один шаг; край 0 и зажим больше 1.
	m.SetStickerFade("s1", st1, 0.3, "f1")
	v = m.SetStickerFade("s1", st1, 7, "f1")
	if got := stickerOf(m, st1).Fade; got != 1 {
		t.Fatalf("прозрачность 7 → %v, ждали зажим 1", got)
	}
	if v2 := m.SetStickerFade("s1", st1, math.Inf(1), "f2"); v2.Rev != v.Rev {
		t.Errorf("Inf в прозрачности — правка")
	}
	m.undo()
	if got := stickerOf(m, st1).Fade; got != 0 {
		t.Fatalf("серия ползунка не одним шагом: %v", got)
	}
	m.redo()

	// Цвета: свой цвет первым, пустой список — цвета генератора.
	m.SetStickerInks("s1", st1, []string{"#123456"}, "p1")
	if got := stickerOf(m, st1).Art.Inks; len(got) == 0 || got[0] != "#123456" {
		t.Fatalf("цвета: %v", got)
	}
	v = m.SetStickerInks("s1", st1, nil, "p2")
	if got := stickerOf(m, st1).Art.Inks; slices.Contains(got, "#123456") {
		t.Errorf("пустой список цветов оставил свой: %v", got)
	}
	if v2 := m.SetStickerInks("s1", st1, nil, "p3"); v2.Rev != v.Rev {
		t.Errorf("те же цвета — правка: rev %d→%d", v.Rev, v2.Rev)
	}
	m.SetStickerInks("s1", st1, []string{"#123456"}, "p4")

	// Неизвестный стикер — холостая; второй стикер не тронут.
	before := ed.View().Rev
	if v2 := m.FlipSticker("s1", "нет", "h"); v2.Rev != before {
		t.Errorf("неизвестный стикер — правка")
	}
	if got := stickerOf(m, st2); got.Art.Kind != other.Art.Kind || got.X != other.X || got.FlipH || got.Fade != 0 {
		t.Errorf("второй стикер изменился: %+v → %+v", other, got)
	}

	// Запись и чтение файла: рамка, зеркало, прозрачность, цвета на месте.
	want := stickerOf(m, st1)
	path := filepath.Join(testkit.TempDir(t), "a.salbum")
	if err := ed.Save(path); err != nil {
		t.Fatal(err)
	}
	p, err := salbum.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	s, i := p.FindSticker(st1)
	if s == nil {
		t.Fatal("стикер пропал после чтения")
	}
	got = s.Stickers[i]
	if got.X != want.X || got.Rot != want.Rot || !got.FlipH || !got.FlipV || got.Fade != want.Fade ||
		got.Art == nil || !slices.Equal(got.Art.Inks, want.Art.Inks) {
		t.Errorf("после чтения %+v, ждали %+v", got, want)
	}
}
