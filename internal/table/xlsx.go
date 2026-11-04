package table

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"path"
	"strings"
)

// readXLSX читает первый видимый лист книги. Ничего не распаковывается на диск (zip-slip
// неприменим), части читаются потоком stream.Token() без Unmarshal: вложенность (≤ MaxDepth) и
// число ячеек не раздувают память. Сравнение — только по Name.Local, поэтому Strict OOXML
// (другие пространства имён) и префиксы читаются так же.
func readXLSX(data []byte) (Sheet, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return Sheet{}, ErrBroken
	}
	if len(zr.File) > MaxEntries {
		return Sheet{}, ErrTooBig
	}
	// Имена частей в Excel и других программах пишутся по-разному (регистр, ведущий «/»).
	files := make(map[string]*zip.File, len(zr.File))
	for _, f := range zr.File {
		files[strings.ToLower(strings.TrimPrefix(f.Name, "/"))] = f
	}
	name, target, count, err := firstSheet(files)
	if err != nil {
		return Sheet{}, err
	}
	var sst []string
	if f := files["xl/sharedstrings.xml"]; f != nil {
		if sst, err = sharedStrings(f); err != nil {
			return Sheet{}, err
		}
	}
	f := files[target]
	if f == nil {
		return Sheet{}, ErrBroken
	}
	rows, err := sheetRows(f, sst)
	if err != nil {
		return Sheet{}, err
	}
	return Sheet{Rows: rows, Name: clean(name), Sheets: count}, nil
}

// limited — как io.LimitReader, но превышение — ошибка, а не тихий конец: обрезанный XML
// мог бы дать «правдоподобную» таблицу. Размеры из заголовка zip не используются (их подделывают).
type limited struct {
	r   io.Reader
	n   int64
	tok int64 // прочитано с начала текущего токена (сбрасывает stream.Token)
}

func (l *limited) Read(p []byte) (int, error) {
	n, err := l.read(p)
	// Токен (тег с миллионом атрибутов, мегабайтный текст) раздувает память Decoder, поэтому
	// он ограничен отдельно от всей части. Decoder читает через bufio по 4 КБ: честный токен
	// короче предела никогда не наберёт столько за один Token().
	if l.tok += int64(n); l.tok > MaxToken && err == nil {
		err = ErrTooBig
	}
	return n, err
}

func (l *limited) read(p []byte) (int, error) {
	if l.n <= 0 {
		// Часть ровно предельного размера — не ошибка: ошибка, только если за ней есть ещё байты.
		var one [1]byte
		n, err := l.r.Read(one[:])
		if n > 0 {
			return 0, ErrTooBig
		}
		return 0, err
	}
	if int64(len(p)) > l.n {
		p = p[:l.n]
	}
	n, err := l.r.Read(p)
	l.n -= int64(n)
	return n, err
}

// wrap — ошибка потока в ошибку пакета: превышение предела остаётся «слишком большая».
func wrap(e error) error {
	if errors.Is(e, ErrTooBig) {
		return ErrTooBig
	}
	return ErrBroken
}

func attr(se xml.StartElement, local string) string {
	for _, a := range se.Attr {
		if a.Name.Local == local {
			return a.Value
		}
	}
	return ""
}

// relTargets читает rels книги: Id → путь части (в нижнем регистре, от корня архива).
func relTargets(f *zip.File) (map[string]string, error) {
	d, c, err := openXML(f, MaxMeta)
	if err != nil {
		return nil, err
	}
	defer c.Close() //nolint:errcheck // чтение: ошибка закрытия ничего не теряет
	targets := map[string]string{}
	for {
		t, e := d.Token()
		if e == io.EOF {
			return targets, nil
		}
		if e != nil {
			return nil, wrap(e)
		}
		if se, ok := t.(xml.StartElement); ok && se.Name.Local == "Relationship" {
			// «/xl/…» — от корня, иначе относительно xl/ (где лежит workbook.xml).
			tg := attr(se, "Target")
			if rest, ok := strings.CutPrefix(tg, "/"); ok {
				tg = rest
			} else {
				tg = path.Join("xl", tg)
			}
			targets[attr(se, "Id")] = strings.ToLower(path.Clean(tg))
		}
	}
}

// firstSheet находит первый видимый лист: его имя, путь части и число всех листов.
func firstSheet(files map[string]*zip.File) (name, target string, count int, err error) {
	wb, rels := files["xl/workbook.xml"], files["xl/_rels/workbook.xml.rels"]
	if wb == nil || rels == nil {
		return "", "", 0, ErrBroken
	}
	targets, err := relTargets(rels)
	if err != nil {
		return "", "", 0, err
	}
	d, c, err := openXML(wb, MaxMeta)
	if err != nil {
		return "", "", 0, err
	}
	defer c.Close() //nolint:errcheck // чтение: ошибка закрытия ничего не теряет
	for {
		t, e := d.Token()
		if e == io.EOF {
			break
		}
		if e != nil {
			return "", "", 0, wrap(e)
		}
		se, ok := t.(xml.StartElement)
		if !ok || se.Name.Local != "sheet" {
			continue
		}
		count++ // листы считаются все, и скрытые, и после найденного
		if target != "" || attr(se, "state") == "hidden" || attr(se, "state") == "veryHidden" {
			continue
		}
		// r:id — атрибут в пространстве relationships (обычное или Strict); Local = "id".
		for _, a := range se.Attr {
			if a.Name.Local == "id" && a.Name.Space != "" {
				target = targets[a.Value]
			}
		}
		if target != "" {
			name = attr(se, "name")
		}
	}
	if target == "" {
		return "", "", count, ErrNoSheet
	}
	return name, target, count, nil
}
