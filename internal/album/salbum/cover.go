package salbum

import (
	"errors"
	"fmt"
	"log"
	"path/filepath"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// Обложка в файле (спека 032, design §4): чтение с зажимом, запись, путь файла-разметки.

// FileCover — обложка в файле: model.Cover целиком (как FilePhoto — PhotoRef), иначе новое
// поле обложки пропало бы при записи. Guide — относительный от папки альбома, GuideAbs —
// запасной абсолютный (как FileFolder.Abs).
type FileCover struct {
	model.Cover
	GuideAbs string `json:"guideAbs,omitempty"`
}

// fixCover — обложка из файла сразу после разворотов, до dropNilPlaces: лист с ID CoverID
// и без полей, которых у обложки нет (FixCoverSheet), числа — ClampCoverSpec, неизвестная
// раскладка — «на лицевой», цвет плашки — #rrggbb или нет, путь разметки — по правилам фото.
// Дальше лист проходит общие шаги Load вместе с разворотами (обходы по Sheets).
// В проекте-листе (Settings.Sheet, 031) обложки нет: она снимается со строкой в log.txt.
func fixCover(p *model.Project, fc *FileCover, base string, v1 bool) {
	if fc != nil && p.Settings.Sheet {
		log.Print("обложка в проекте-листе снята: у листа обложки нет")
		fc = nil
	}
	if fc == nil {
		p.Cover = nil
		return
	}
	c := fc.Cover
	if c.Sheet == nil {
		c.Sheet = &model.Spread{}
	}
	model.FixCoverSheet(c.Sheet)
	c.Spec = model.ClampCoverSpec(c.Spec)
	if !model.ValidCoverLayout(c.Layout) {
		c.Layout = model.CoverFront
	}
	c.SpineBg = model.BgColor(c.SpineBg)
	c.Guide, c.GuideAlt = loadGuide("обложка", c.Guide, fc.GuideAbs, base, v1)
	p.Cover = &c
}

// fixCoverAndGuide — обложка (fixCover) и путь разметки разворотов (066): обе разметки
// читаются по одним правилам (loadGuide).
func fixCoverAndGuide(p *model.Project, f *FileProject, base string, v1 bool) {
	fixCover(p, f.Cover, base, v1)
	p.SpreadGuide, p.SpreadGuideAlt = loadGuide("развороты", f.SpreadGuide, f.SpreadGuideAbs, base, v1)
}

// guideExt — разметка типографии — PSD (066), JPEG или PNG.
func guideExt(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".jpg", ".jpeg", ".png", ".psd":
		return true
	}
	return false
}

// loadGuide — путь разметки из файла: те же candidates, что у фото (относительный от папки
// альбома, запасной abs; CheckRel и paths.CheckLocalPath). Основного файла нет, а запасной
// есть — запасной; нет нигде — основной, а запасной едет в alt (Cover.GuideAlt), как у фото.
// Недопустимый путь или расширение — "" и строка в log.txt без пути: разметка только
// подложка, её выбирают заново, а открыть альбом важнее. what — чья разметка, для log.txt.
func loadGuide(what, path, abs, base string, v1 bool) (guide, alt string) {
	if path == "" {
		return "", ""
	}
	primary, alt, err := candidates(path, abs, base, v1)
	if err != nil || !guideExt(primary) {
		log.Printf("%s: файл разметки отброшен — недопустимый путь или тип", what)
		return "", ""
	}
	if alt == "" || !guideExt(alt) {
		return primary, ""
	}
	if _, ok := fileSize(primary); ok {
		return primary, ""
	}
	if _, ok := fileSize(alt); ok {
		return alt, ""
	}
	return primary, alt
}

// encodeCover — обложка для записи: путь разметки внутри base — относительный с абсолютным
// рядом; base == "" (черновик) — абсолютный. Без обложки поле не пишется (omitempty).
func encodeCover(c *model.Cover, base string) *FileCover {
	if c == nil {
		return nil
	}
	fc := &FileCover{Cover: *c}
	fc.GuideAlt = "" // в файле запасной путь — guideAbs
	fc.Guide, fc.GuideAbs = encodeGuide(c.Guide, c.GuideAlt, base)
	return fc
}

// encodeGuide — путь разметки для записи: внутри base — относительный с абсолютным рядом;
// base == "" (черновик) — абсолютный; разметки не нашлось при открытии — запасной как был.
func encodeGuide(guide, alt, base string) (path, abs string) {
	if guide == "" {
		return "", ""
	}
	path = guide
	if r, ok := RelPath(base, guide); ok {
		path, abs = r, guide
	}
	if alt != "" {
		abs = alt
	}
	return path, abs
}

// coverNoAuto — у обложки нет «Авто»: если починка мест (SyncGrid(true)) сняла сетку
// (мест не хватило), фото листа уходят с обложки, а записи без дела — PruneRefs.
func coverNoAuto(p *model.Project) {
	if p.Cover == nil || len(p.Cover.Sheet.Grid) > 0 || len(p.Cover.Sheet.Photos) == 0 {
		return
	}
	p.Cover.Sheet.Photos = nil
	p.PruneRefs()
}

// relinkGuides — смена буквы диска для разметок обложки и разворотов (RelinkDrive). Relink по
// файлам фото разметку не трогает: она не фото.
func relinkGuides(p *model.Project, from, to string) {
	if p.Cover != nil {
		relinkGuide(&p.Cover.Guide, &p.Cover.GuideAlt, from, to)
	}
	relinkGuide(&p.SpreadGuide, &p.SpreadGuideAlt, from, to)
}

// relinkGuide — файла *guide нет на месте, а на диске to есть: путь переезжает, запасной снят.
func relinkGuide(guide, alt *string, from, to string) {
	if *guide == "" {
		return
	}
	if _, here := fileSize(*guide); here {
		return
	}
	if np, ok := paths.SwapDrive(*guide, from, to); ok {
		if _, there := fileSize(np); there {
			*guide, *alt = np, ""
		}
	}
}

// CheckGuide — путь файла-разметки из интерфейса или диалога (SetCoverGuide, SetSpreadGuide):
// абсолютный локальный (paths.CheckLocalPath: без UNC и устройств) и только PSD, JPEG или PNG;
// PSB — свой текст (066 №11).
func CheckGuide(path string) error {
	if err := paths.CheckLocalPath(path); err != nil {
		return fmt.Errorf("файл разметки: %w", err)
	}
	if strings.EqualFold(filepath.Ext(path), ".psb") {
		return library.ErrPSB
	}
	if !guideExt(path) {
		return errors.New("файл разметки — только PSD, JPEG или PNG")
	}
	return nil
}

// GuideInfo — сведения о файле-разметке (library.MarkupInfo: размер, у PSD — DPI и
// направляющие); ok=false — файла нет, он не читается или том не отвечает (мёртвый сетевой
// диск не вешает вид, 019).
func GuideInfo(path string) (library.MarkupInfo, bool) {
	if path == "" || !reachable(path) {
		return library.MarkupInfo{}, false
	}
	m, err := library.MarkupFile(path)
	if err != nil {
		return library.MarkupInfo{}, false
	}
	return m, true
}
