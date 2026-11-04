// Package psd — узкий разбор PSD (Photoshop v1) для разметки типографии (спека 066):
// заголовок, ресурсы 1005 (dpi), 1032 (направляющие), 1057 (есть ли сведённое) и сведённое
// изображение, уменьшенное потоком. Слои, текст, смарт-объекты, ICC и XMP не разбираются.
// Только стандартная библиотека; вход недоверенный (PSD из интернета): каждая длина из файла
// сверяется с остатком файла и пределами до выделения памяти, произведения — в int64.
package psd

import (
	"encoding/binary"
	"errors"
	"io"
)

// Ошибки разбора. Тексты показываются пользователю (карточка разметки, тост); с маленькой
// буквы — требование revive, вызывающий при показе поднимает первую букву.
var (
	ErrPSB      = errors.New("файл PSB (большой документ Photoshop) не поддерживается — сохраните его как PSD")
	ErrNotPSD   = errors.New("это не файл PSD")
	ErrCorrupt  = errors.New("файл PSD повреждён")
	ErrSide     = errors.New("PSD больше 30000 px по стороне — не поддерживается")
	ErrPixels   = errors.New("PSD больше 250 Мп — не поддерживается")
	ErrFileSize = errors.New("файл разметки больше 2 ГБ")
)

// Пределы (design §2.4).
const (
	maxSidePx     = 30000       // предел PSD v1
	maxPixels     = 250_000_000 // как imgfile.MaxPixels; пакет от imgfile не зависит
	maxChannels   = 56
	maxFileSize   = 2 << 30 // предел PSD v1
	maxColorData  = 1 << 20
	maxGuides     = 1000
	maxGuideBlock = 16 + 5*maxGuides
	maxBlocks     = 4096 // блоков ресурсов: в живом PSD десятки; больше — разбор ресурсов останавливается
	headerSize    = 26
)

// Режимы цвета (поле Mode); Lab — последний известный.
const (
	modeGray = 1
	modeRGB  = 3
	modeCMYK = 4
	modeLab  = 9
)

// Тексты Info.Note (design §2.4).
const (
	noteNoMerged = "В PSD нет сведённого изображения — видны только направляющие. Сохраните PSD с «Максимальной совместимостью»"
	noteColor    = "Цвет PSD (Lab, 32 бит…) не поддерживается — видны только направляющие"
)

// Guide — направляющая. Pos в px документа (в файле — 1/32 px), Horz — горизонтальная (положение по Y).
type Guide struct {
	Pos  float64
	Horz bool
}

// Info — заголовок и ресурсы файла.
type Info struct {
	W, H, Channels, Depth, Mode int
	DPI                         float64 // ресурс 1005, ppi; 0 — нет ресурса или вне 1…10000
	Guides                      []Guide // ресурс 1032; вне [0, W] или [0, H] — отброшены
	Merged                      bool    // ресурс 1057 hasRealMergedData; нет ресурса — true
	Image                       bool    // сведённое можно показать (Merged, режим, глубина, сжатие, каналы)
	Note                        string  // "" или почему картинки нет
}

// doc — Info плюс то, что нужно Preview: сжатие и смещение данных изображения (после u16 сжатия).
type doc struct {
	Info
	comp   int
	imgOff int64
}

// ReadInfo читает заголовок и ресурсы; данные изображения не читает. size — размер файла (Stat).
func ReadInfo(r io.ReadSeeker, size int64) (Info, error) {
	d, err := parse(r, size)
	if err != nil {
		return Info{}, err
	}
	return d.Info, nil
}

// rd — чтение по смещению с контролем остатка файла. Чтение за концом — ErrCorrupt.
type rd struct {
	r    io.Reader
	sk   io.Seeker // nil в потоке изображения: там skip не нужен
	size int64
	pos  int64
}

func (x *rd) full(b []byte) error {
	if int64(len(b)) > x.size-x.pos {
		return ErrCorrupt
	}
	n, err := io.ReadFull(x.r, b)
	x.pos += int64(n)
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return ErrCorrupt
	}
	return err
}

func (x *rd) u32() (int64, error) {
	var b [4]byte
	if err := x.full(b[:]); err != nil {
		return 0, err
	}
	return int64(binary.BigEndian.Uint32(b[:])), nil
}

// skip переходит вперёд на n байт; за конец файла — ErrCorrupt.
func (x *rd) skip(n int64) error {
	if n < 0 || n > x.size-x.pos {
		return ErrCorrupt
	}
	if n == 0 {
		return nil
	}
	if x.sk == nil {
		return errors.New("psd: переход в потоке")
	}
	p, err := x.sk.Seek(n, io.SeekCurrent)
	if err != nil {
		return err
	}
	x.pos = p
	return nil
}

func parse(r io.ReadSeeker, size int64) (doc, error) {
	var d doc
	if size > maxFileSize {
		return d, ErrFileSize
	}
	x := &rd{r: r, sk: r, size: size}
	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return d, err
	}
	if err := d.header(x); err != nil {
		return d, err
	}
	// Color mode data: палитра/дуотон не нужны.
	n, err := x.u32()
	if err != nil {
		return d, err
	}
	if n > maxColorData {
		return d, ErrCorrupt
	}
	if err := x.skip(n); err != nil {
		return d, err
	}
	d.Merged = true
	if err := d.resources(x); err != nil {
		return d, err
	}
	// Layer and mask info: слои не читаем.
	if n, err = x.u32(); err != nil {
		return d, err
	}
	if err := x.skip(n); err != nil {
		return d, err
	}
	var cb [2]byte
	if err := x.full(cb[:]); err != nil {
		return d, err
	}
	d.comp = int(binary.BigEndian.Uint16(cb[:]))
	d.imgOff = x.pos
	d.classify()
	return d, nil
}

func (d *doc) header(x *rd) error {
	var h [headerSize]byte
	if err := x.full(h[:]); err != nil {
		if errors.Is(err, ErrCorrupt) {
			return ErrNotPSD // короче заголовка: подписи нет
		}
		return err
	}
	if string(h[:4]) != "8BPS" {
		return ErrNotPSD
	}
	switch binary.BigEndian.Uint16(h[4:]) {
	case 1:
	case 2:
		return ErrPSB
	default:
		return ErrNotPSD
	}
	h32, w32 := binary.BigEndian.Uint32(h[14:]), binary.BigEndian.Uint32(h[18:])
	d.Channels = int(binary.BigEndian.Uint16(h[12:]))
	d.Depth = int(binary.BigEndian.Uint16(h[22:]))
	d.Mode = int(binary.BigEndian.Uint16(h[24:]))
	if h32 > maxSidePx || w32 > maxSidePx {
		return ErrSide
	}
	d.H, d.W = int(h32), int(w32)
	if d.Channels < 1 || d.Channels > maxChannels || d.W < 1 || d.H < 1 || d.Mode > modeLab {
		return ErrCorrupt
	}
	switch d.Depth {
	case 1, 8, 16, 32:
	default:
		return ErrCorrupt
	}
	if int64(d.W)*int64(d.H) > maxPixels {
		return ErrPixels
	}
	return nil
}

// needChannels — сколько каналов нужно для показа; 0 — режим не поддержан.
func needChannels(mode int) int {
	switch mode {
	case modeGray:
		return 1
	case modeRGB:
		return 3
	case modeCMYK:
		return 4
	}
	return 0
}

// classify решает, можно ли показать сведённое, и пишет Note.
func (d *doc) classify() {
	switch {
	case !d.Merged:
		d.Note = noteNoMerged
	case needChannels(d.Mode) == 0, d.Depth != 8 && d.Depth != 16, d.comp != 0 && d.comp != 1,
		d.Channels < needChannels(d.Mode):
		d.Note = noteColor
	default:
		d.Image = true
	}
}

// resources читает секцию Image resources блок за блоком; остальные блоки пропускаются Seek.
// Неизвестная подпись или слишком много блоков — конец разбора ресурсов без ошибки.
func (d *doc) resources(x *rd) error {
	n, err := x.u32()
	if err != nil {
		return err
	}
	if n > x.size-x.pos {
		return ErrCorrupt
	}
	end := x.pos + n
	seenGuides := false
	for blocks := 0; end-x.pos >= 12 && blocks < maxBlocks; blocks++ {
		var h [7]byte // подпись, id, длина Pascal-имени
		if err := x.full(h[:]); err != nil {
			return err
		}
		if !knownSig(h[:4]) {
			break
		}
		id := binary.BigEndian.Uint16(h[4:])
		name := int64(h[6]) + 1 // Pascal-строка с байтом длины, выровненная до чётного
		name += name % 2
		if err := x.skip(name - 1); err != nil {
			return err
		}
		if end-x.pos < 4 {
			break
		}
		sz, err := x.u32()
		if err != nil {
			return err
		}
		if sz > end-x.pos {
			return ErrCorrupt
		}
		next := min(x.pos+sz+sz%2, end)
		switch {
		case id == 1005 && sz >= 16:
			err = d.readDPI(x)
		case id == 1032 && !seenGuides && sz >= 16 && sz <= maxGuideBlock:
			seenGuides = true
			err = d.readGuides(x, int(sz))
		case id == 1057 && sz >= 5:
			var b [5]byte
			if err = x.full(b[:]); err == nil {
				d.Merged = b[4] != 0
			}
		}
		if err != nil {
			return err
		}
		if err := x.skip(next - x.pos); err != nil {
			return err
		}
	}
	return x.skip(end - x.pos)
}

func knownSig(s []byte) bool {
	switch string(s) {
	case "8BIM", "MeSa", "AgHg", "PHUT", "DCSR":
		return true
	}
	return false
}

// readDPI — ресурс 1005: hRes Fixed 16.16 в пикселях на дюйм. Единица (ppi/ppcm) — лишь то, в чём
// Photoshop показывает разрешение: по Adobe Photoshop File Formats (ResolutionInfo: hRes — pixels
// per inch) и GIMP (psd-image-res-load: «Resolution always recorded as pixels / inch») число в
// файле всегда ppi, умножать на 2,54 при единице 2 нельзя.
func (d *doc) readDPI(x *rd) error {
	var b [6]byte
	if err := x.full(b[:]); err != nil {
		return err
	}
	v := float64(binary.BigEndian.Uint32(b[:4])) / 65536
	if v >= 1 && v <= 10000 {
		d.DPI = v
	}
	return nil
}

// readGuides — ресурс 1032 размером sz (16 ≤ sz ≤ maxGuideBlock). Число направляющих больше
// предела или не влезающее в блок — ресурс пропускается; вне документа — отбрасываются.
func (d *doc) readGuides(x *rd, sz int) error {
	b := make([]byte, sz)
	if err := x.full(b); err != nil {
		return err
	}
	n := int64(binary.BigEndian.Uint32(b[12:]))
	if n > maxGuides || 16+5*n > int64(sz) {
		return nil
	}
	for i := range int(n) {
		o := 16 + 5*i
		raw := int64(binary.BigEndian.Uint32(b[o:])) // i32 со знаком: ≥ 2³¹ — отрицательное
		if raw >= 1<<31 {
			raw -= 1 << 32
		}
		pos := float64(raw) / 32
		var g Guide
		switch b[o+4] {
		case 0:
			g = Guide{Pos: pos}
		case 1:
			g = Guide{Pos: pos, Horz: true}
		default:
			continue
		}
		lim := d.W
		if g.Horz {
			lim = d.H
		}
		if pos >= 0 && pos <= float64(lim) {
			d.Guides = append(d.Guides, g)
		}
	}
	return nil
}
