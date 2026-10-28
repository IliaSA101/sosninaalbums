package imgfile

import (
	"encoding/binary"
	"errors"
	"fmt"
	"image/color"
	"io"
)

// Пределы обхода RIFF-чанков WebP до кадра: файл с тысячами пустых чанков или с гигабайтным
// «служебным» чанком не должен заставлять заголовок читаться вечно.
const (
	webpMaxChunks = 32
	webpMaxSkip   = 256 << 20 // суммарно пропускаемых байт до кадра (ALPH большого кадра — единицы МБ)
)

var (
	errWebPBroken = errors.New("повреждённый заголовок WebP")
	errWebPAnim   = errors.New("анимированный WebP не поддерживается")
	errICCTooBig  = errors.New("профиль ICCP WebP больше предела")
)

// webpHead — размер кадра и модель декода WebP по заголовкам чанков.
type webpHead struct {
	w, h  int
	model color.Model // YCbCr — lossy, NYCbCrA — lossy с ALPH, NRGBA — lossless (VP8L)
}

// webpWalk — состояние обхода чанков.
type webpWalk struct {
	vp8x         bool
	canvW, canvH int
	alpha        bool
	lossless     bool
	found        bool
	w, h         int
	skipped      int64
	wantICC      bool   // Header: брать ICCP (DimsOf профиль не читает)
	flagICC      bool   // в VP8X стоит флаг ICC
	icc          []byte // первый ICCP при флаге
	haveICC      bool
	iccBad       bool // ICCP заявлен, но длиннее iccMaxBytes
}

// readWebPHead обходит чанки RIFF (r стоит сразу за 12-байтным заголовком RIFF/WEBP) до первого
// кадра VP8/VP8L. Анимация (VP8X с флагом, ANIM, ANMF) и VP8X без кадра — отказ; размер кадра
// обязан совпасть с холстом VP8X (Decode проверяет то же, но только после выделения памяти).
func readWebPHead(r io.Reader) (webpHead, error) {
	var wk webpWalk
	if err := wk.walk(r); err != nil {
		return webpHead{}, err
	}
	if wk.w <= 0 || wk.h <= 0 {
		return webpHead{}, fmt.Errorf("некорректный размер изображения %dx%d", wk.w, wk.h)
	}
	if wk.vp8x && (wk.w != wk.canvW || wk.h != wk.canvH) {
		return webpHead{}, errors.New("размер кадра WebP не совпадает с холстом")
	}
	h := webpHead{w: wk.w, h: wk.h, model: color.YCbCrModel}
	switch {
	case wk.lossless:
		h.model = color.NRGBAModel
	case wk.alpha:
		h.model = color.NYCbCrAModel
	}
	return h, nil
}

// walk обходит чанки до первого кадра (не больше webpMaxChunks).
func (wk *webpWalk) walk(r io.Reader) error {
	for i := 0; i < webpMaxChunks && !wk.found; i++ {
		if err := wk.chunk(r, i == 0); err != nil {
			return err
		}
	}
	if !wk.found {
		return errors.New("в WebP нет кадра")
	}
	return nil
}

// webpICCHead — Head WebP для Header: класс профиля из ICCP (только при флаге ICC в VP8X, как у Chromium).
// Ошибка обхода (анимация, битый чанк, нет кадра) — «без профиля»: такой файл DimsOf и так отвергает.
func webpICCHead(r io.Reader) Head {
	h := Head{Orient: 1}
	wk := webpWalk{wantICC: true}
	err := wk.walk(r)
	switch {
	case wk.iccBad: // профиль заявлен, но не прочитан: обход остановлен на нём
		h.Color, h.Profile = ColorOther, iccUnknownName
	case err == nil && wk.icc != nil:
		h.Color, h.Profile, h.xf = classify(wk.icc)
	}
	return h
}

// chunk читает один чанк: заголовок кадра разбирается, остальные пропускаются.
func (wk *webpWalk) chunk(r io.Reader, first bool) error {
	var hdr [8]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return errWebPBroken
	}
	n := int64(binary.LittleEndian.Uint32(hdr[4:]))
	switch string(hdr[:4]) {
	case "ANIM", "ANMF":
		return errWebPAnim
	case "VP8X":
		return wk.readVP8X(r, first, n)
	case "VP8 ":
		return wk.readVP8(r, n)
	case "VP8L":
		return wk.readVP8L(r, n)
	case "ALPH":
		wk.alpha = true
	case "ICCP":
		if wk.wantICC && wk.flagICC && !wk.haveICC {
			return wk.readICCP(r, n)
		}
	}
	wk.skipped += n + n&1 // данные выровнены до чётного
	if wk.skipped > webpMaxSkip {
		return errors.New("слишком длинные служебные чанки WebP")
	}
	if err := skipBytes(r, n+n&1); err != nil {
		return errWebPBroken
	}
	return nil
}

// readICCP — профиль первого ICCP при флаге ICC (VP8X уже прочитан). Сначала длина: больше
// iccMaxBytes — ColorOther без чтения (обход на этом останавливается), иначе читается целиком вместе
// с байтом выравнивания.
func (wk *webpWalk) readICCP(r io.Reader, n int64) error {
	wk.haveICC = true
	if n > iccMaxBytes {
		wk.iccBad = true
		return errICCTooBig
	}
	wk.skipped += n + n&1
	buf := make([]byte, n+n&1)
	if _, err := io.ReadFull(r, buf); err != nil {
		return errWebPBroken
	}
	wk.icc = buf[:n]
	return nil
}

// readVP8X — холст и флаги; VP8X бывает только первым чанком и ровно в 10 байт.
func (wk *webpWalk) readVP8X(r io.Reader, first bool, n int64) error {
	var b [10]byte
	if !first || n != int64(len(b)) {
		return errWebPBroken
	}
	if _, err := io.ReadFull(r, b[:]); err != nil {
		return errWebPBroken
	}
	if b[0]&vp8xAnimation != 0 {
		return errWebPAnim
	}
	wk.vp8x = true
	wk.alpha = b[0]&vp8xAlpha != 0
	wk.flagICC = b[0]&vp8xICC != 0
	wk.canvW = (int(b[4]) | int(b[5])<<8 | int(b[6])<<16) + 1
	wk.canvH = (int(b[7]) | int(b[8])<<8 | int(b[9])<<16) + 1
	return nil
}

// readVP8 — кадровый заголовок lossy: тег (бит 0 — «не ключевой»), стартовый код 9D 01 2A,
// затем 14-битные стороны.
func (wk *webpWalk) readVP8(r io.Reader, n int64) error {
	var b [10]byte
	if n < int64(len(b)) {
		return errWebPBroken
	}
	if _, err := io.ReadFull(r, b[:]); err != nil {
		return errWebPBroken
	}
	if b[0]&1 != 0 || b[3] != 0x9d || b[4] != 0x01 || b[5] != 0x2a {
		return errWebPBroken
	}
	wk.w = int(binary.LittleEndian.Uint16(b[6:]) & 0x3fff)
	wk.h = int(binary.LittleEndian.Uint16(b[8:]) & 0x3fff)
	wk.found = true
	return nil
}

// readVP8L — заголовок lossless: сигнатура 2F и 14-битные стороны минус один.
func (wk *webpWalk) readVP8L(r io.Reader, n int64) error {
	var b [5]byte
	if n < int64(len(b)) {
		return errWebPBroken
	}
	if _, err := io.ReadFull(r, b[:]); err != nil {
		return errWebPBroken
	}
	if b[0] != 0x2f {
		return errWebPBroken
	}
	bits := binary.LittleEndian.Uint32(b[1:])
	wk.w = int(bits&0x3fff) + 1
	wk.h = int(bits>>14&0x3fff) + 1
	wk.lossless = true
	wk.found = true
	return nil
}

// skipBytes пропускает n байт: Seek, если источник умеет (файл), иначе чтение в никуда.
// Seek за конец файла ошибки не даёт — следующее чтение вернёт EOF.
func skipBytes(r io.Reader, n int64) error {
	if s, ok := r.(io.Seeker); ok {
		_, err := s.Seek(n, io.SeekCurrent)
		return err
	}
	_, err := io.CopyN(io.Discard, r, n)
	return err
}
