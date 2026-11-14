package decor

import (
	"bytes"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"log"
	"path/filepath"
	"strconv"

	"github.com/IliaSA101/sosninaalbums/internal/imgfile"
	"github.com/IliaSA101/sosninaalbums/internal/library"
)

// Миграция признака QR (0.28-131-215): коды ссылки 0.23–0.27, переименованные фотографом, признака
// не имеют (isQR узнаёт только по имени «QR · …»). При открытии библиотеки такие стикеры узнаются по
// содержимому PNG — это картинка, которую рисует QR.PNG, — и получают признак в реестре.

// Версия миграции (qrScan в library.json) и пределы разбора.
const (
	qrScanV     = 1       // записано — стикеры уже просмотрены; новее программы — не трогаем
	qrScanFile  = 1 << 20 // PNG кода — десятки КБ (два цвета, блоки одного размера); больше — не наш
	qrMinSide   = 21      // модулей у версии 1
	qrMaxVer    = 40      // выше версии QR нет
	qrFinderLen = 7       // сторона узора-«глазка», модулей
)

// migrateQR — одноразовая проверка своих стикеров без признака: узнанным ставит qr в реестре.
// Реестр новее программы или не читается (readOnly), версия миграции уже стоит — ничего не делает.
// Файл правится точечно (qr у записей и qrScan), как writeBase: остальное переписывается как есть.
// Узнанные стикеры помечаются в памяти и при сбое записи — до конца сеанса код клиента закрыт.
func (s *Store) migrateQR() {
	s.ioMu.Lock()
	defer s.ioMu.Unlock()
	s.mu.RLock()
	skip := s.readOnly || s.qrScan >= qrScanV
	var todo []*entry  // без признака и без «QR · » в имени: смотрим файл
	var named []*entry // старые записи с именем «QR · …»: признак не стоит, файл смотреть незачем
	paths := map[*entry]string{}
	if !skip {
		for _, e := range s.entries {
			if e.Kind != KindSticker || e.QR {
				continue
			}
			if e.isQR() {
				named = append(named, e)
			} else if p, ok := s.files[KindSticker][e.Key]; ok && e.Ext == "png" {
				todo = append(todo, e)
				paths[e] = p
			}
		}
	}
	s.mu.RUnlock()
	if skip {
		return
	}
	found := map[string]bool{}
	for _, e := range named { // иначе первое же ✎ сняло бы с записи единственный след кода
		found[e.Key] = true
	}
	for _, e := range todo {
		if isQRFile(paths[e]) {
			found[e.Key] = true
		}
	}
	s.mu.Lock()
	for _, e := range append(todo, named...) {
		if found[e.Key] {
			e.QR = true
		}
	}
	s.mu.Unlock()
	if err := s.writeQRScan(found); err != nil {
		log.Printf("библиотека: признак QR не записан: %v", err)
		return
	}
	s.mu.Lock()
	s.qrScan = qrScanV
	s.mu.Unlock()
	if len(found) > 0 {
		log.Printf("библиотека: старых кодов ссылки узнано (по имени и содержимому): %d", len(found))
	}
}

// writeQRScan — меняет в library.json на диске только qr у стикеров found и qrScan; читает текущий файл,
// не память со старта. Нет файла, он больше maxRegSize, не JSON-объект, версия не наша — не пишет
// (не ошибка: узнанное остаётся в памяти).
func (s *Store) writeQRScan(found map[string]bool) error {
	path := filepath.Join(s.root, regFile)
	data, err := readCapped(path, maxRegSize)
	if err != nil {
		if errors.Is(err, errTooBig) {
			return nil
		}
		return err
	}
	var m map[string]json.RawMessage
	if json.Unmarshal(data, &m) != nil {
		return nil
	}
	var v int
	if json.Unmarshal(m["version"], &v) != nil || v < 1 || v > regVersion {
		return nil // новее программы (или нет вовсе): чужое не перезаписываем
	}
	var items []json.RawMessage
	if json.Unmarshal(m["items"], &items) != nil {
		return nil
	}
	for i, raw := range items {
		var it map[string]json.RawMessage
		var key, kind string
		if json.Unmarshal(raw, &it) != nil || json.Unmarshal(it["key"], &key) != nil || json.Unmarshal(it["kind"], &kind) != nil {
			continue
		}
		if kind == KindSticker && found[key] {
			it["qr"] = json.RawMessage("true")
			if items[i], err = plainJSON(it); err != nil {
				return err
			}
		}
	}
	if m["items"], err = plainJSON(items); err != nil {
		return err
	}
	m["qrScan"] = json.RawMessage(strconv.Itoa(qrScanV))
	var out bytes.Buffer
	enc := json.NewEncoder(&out)
	enc.SetEscapeHTML(false) // чужие значения остаются теми же символами
	enc.SetIndent("", "  ")
	if err := enc.Encode(m); err != nil {
		return err
	}
	return library.WriteFileAtomic(path, bytes.TrimRight(out.Bytes(), "\n"))
}

// plainJSON — Marshal без экранирования <, >, &: json.Marshal экранирует их и внутри RawMessage,
// и чужие значения превратились бы в \u003c.
func plainJSON(v any) ([]byte, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(b.Bytes(), "\n"), nil
}

// isQRFile — файл path — картинка кода, как её рисует QR.PNG: квадрат из двух цветов, сторона —
// целое число модулей версии 1…40 с белым полем в QRQuiet модулей, каждый модуль — однотонный
// квадрат px×px, три «глазка» в углах и линии синхронизации между ними. Декодировать код не нужно:
// такой рисунок обычным стикером почти не бывает, а ложное срабатывание (чужой QR, загруженный
// как стикер) лишь не пустит стикер в макет — безопасная сторона. Размер файла и сторона
// проверяются до декода; нечитаемое, битое, большое — false.
func isQRFile(path string) bool {
	f, err := imgfile.OpenRegular(path)
	if err != nil {
		return false
	}
	defer f.Close() //nolint:errcheck // чтение: ошибка закрытия ничего не теряет
	if st, err := f.Stat(); err != nil || st.Size() > qrScanFile {
		return false
	}
	return isQRStream(f)
}

// isQRStream — isQRFile по открытому файлу: сначала только шапка PNG (DecodeConfig читает до первых
// данных): квадрат, палитра чёрный+белый, сторона; чужой рисунок отпадает, не прочитавшись целиком.
func isQRStream(r io.ReadSeeker) bool {
	cfg, err := png.DecodeConfig(r)
	if err != nil {
		return false
	}
	if _, ok := qrHead(cfg); !ok {
		return false
	}
	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return false
	}
	data, err := io.ReadAll(io.LimitReader(r, qrScanFile+1))
	return err == nil && isQRPNG(data)
}

// qrHead — шапка PNG подходит под QR.PNG: квадрат нужной стороны, палитра из чёрного и белого;
// возвращает индекс тёмного цвета.
func qrHead(cfg image.Config) (uint8, bool) {
	if cfg.Width != cfg.Height || cfg.Width < qrMinSide+2*QRQuiet || cfg.Width > MaxStickerSide {
		return 0, false
	}
	pal, ok := cfg.ColorModel.(color.Palette)
	if !ok || len(pal) != 2 {
		return 0, false
	}
	return darkIndex(pal)
}

// isQRPNG — isQRFile по байтам файла (его вход и фаззится): больше qrScanFile — false.
func isQRPNG(data []byte) bool {
	if len(data) > qrScanFile {
		return false
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return false
	}
	dark, ok := qrHead(cfg)
	if !ok {
		return false
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return false
	}
	p, ok := img.(*image.Paletted)
	if !ok || p.Palette[dark] == nil {
		return false
	}
	return qrGridOf(p, dark)
}

// darkIndex — какой из двух цветов палитры тёмный: чёрный и белый, непрозрачные.
func darkIndex(pal color.Palette) (uint8, bool) {
	isC := func(c color.Color, v uint32) bool {
		r, g, b, a := c.RGBA()
		return r == v && g == v && b == v && a == 0xffff
	}
	switch {
	case isC(pal[0], 0xffff) && isC(pal[1], 0):
		return 1, true
	case isC(pal[0], 0) && isC(pal[1], 0xffff):
		return 0, true
	}
	return 0, false
}

// qrGridOf — картинка p укладывается в сетку модулей какой-то версии QR (модуль — mod px).
func qrGridOf(p *image.Paletted, dark uint8) bool {
	w := p.Bounds().Dx()
	for ver := 1; ver <= qrMaxVer; ver++ {
		n := 17 + 4*ver
		side := n + 2*QRQuiet
		if w%side != 0 {
			continue
		}
		mod := w / side
		at := func(mx, my int) bool { return p.Pix[(my*mod)*p.Stride+mx*mod] == dark } // выборка модуля
		if qrPattern(at, n) && qrBlocksUniform(p, mod, side) {
			return true
		}
	}
	return false
}

// qrPattern — по выборке модулей at(mx, my) (с полем): поле светлое, три «глазка»,
// синхролинии между ними.
func qrPattern(at func(mx, my int) bool, n int) bool {
	side := n + 2*QRQuiet
	for i := range side {
		for j := range QRQuiet {
			if at(i, j) || at(i, side-1-j) || at(j, i) || at(side-1-j, i) {
				return false
			}
		}
	}
	q := QRQuiet
	for _, o := range [][2]int{{q, q}, {q + n - qrFinderLen, q}, {q, q + n - qrFinderLen}} {
		for dy := range qrFinderLen {
			for dx := range qrFinderLen {
				ring := dx == 0 || dy == 0 || dx == qrFinderLen-1 || dy == qrFinderLen-1
				core := dx >= 2 && dx <= 4 && dy >= 2 && dy <= 4
				if at(o[0]+dx, o[1]+dy) != (ring || core) {
					return false
				}
			}
		}
	}
	for i := 8; i <= n-9; i++ { // между «глазками»: тёмный, светлый, … с чётного модуля
		if at(q+i, q+6) != (i%2 == 0) || at(q+6, q+i) != (i%2 == 0) {
			return false
		}
	}
	return true
}

// qrBlocksUniform — каждый модуль (mod×mod px) одного индекса палитры: на сжатом, размытом или
// масштабированном рисунке выборка по углу блока обманула бы.
func qrBlocksUniform(p *image.Paletted, mod, side int) bool {
	for my := range side {
		for mx := range side {
			want := p.Pix[(my*mod)*p.Stride+mx*mod]
			for y := my * mod; y < (my+1)*mod; y++ {
				row := p.Pix[y*p.Stride+mx*mod : y*p.Stride+(mx+1)*mod]
				for _, v := range row {
					if v != want {
						return false
					}
				}
			}
		}
	}
	return true
}
