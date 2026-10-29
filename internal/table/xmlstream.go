package table

import (
	"archive/zip"
	"encoding/xml"
	"io"
)

// Пределы разбора XML: вложенность и размер одного токена. Общий предел части (MaxPart) их не
// заменяет: 5 млн вложенных <a> или миллион атрибутов в одном теге укладываются в 16 МБ,
// но раздувают стек пространств имён и срез атрибутов Decoder в сотни МБ.
const (
	MaxDepth = 64      // открытых элементов одновременно: у листа Excel их меньше десяти
	MaxToken = 1 << 19 // байт одного токена (тег с атрибутами, текст между тегами): ячейка Excel ≤ 32767 знаков, ~100 КБ
)

// stream — xml.Decoder, у которого Token() следит за глубиной и размером токена. Превышение
// глубины — ErrBroken (настоящий файл так не устроен), размера токена — ErrTooBig.
type stream struct {
	d     *xml.Decoder
	lim   *limited
	depth int
}

// Token — следующий токен потока; единственный способ читать XML в пакете.
func (s *stream) Token() (xml.Token, error) {
	s.lim.tok = 0 // предел токена считается заново с каждого Token()
	t, err := s.d.Token()
	switch t.(type) {
	case xml.StartElement:
		if s.depth++; s.depth > MaxDepth {
			return nil, ErrBroken
		}
	case xml.EndElement:
		s.depth--
	}
	return t, err
}

// openXML открывает часть zip как поток XML с пределом распакованного размера.
func openXML(f *zip.File, limit int64) (*stream, io.Closer, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, nil, ErrBroken
	}
	l := &limited{r: rc, n: limit}
	return &stream{d: xml.NewDecoder(l), lim: l}, rc, nil
}
