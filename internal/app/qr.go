package app

import (
	"log"
	"unicode/utf8"

	"github.com/IliaSA101/sosninaalbums/internal/decor"
)

// QR-код по ссылке (спека 100, п. 2): образец для окна и постановка стикером. Ссылка — личные
// данные: в лог идёт только длина; ссылку программа не открывает и путём не считает.

// QRInfo — образец кода для окна: SVG-путь рисует UI в viewBox="0 0 side side".
type QRInfo struct {
	Text  string  `json:"text"`  // что будет в коде (после дополнения «https://»)
	Side  int     `json:"side"`  // модулей с белым полем
	Path  string  `json:"path"`  // d тёмных модулей
	MinMM float64 `json:"minMM"` // печатать не мельче, мм
}

// QRPreview — образец для окна, без записи. Ошибка — текст строки под полем.
func (a *App) QRPreview(link string) (info QRInfo, err error) {
	defer guardErr("QRPreview", &err)
	q, err := decor.EncodeQR(link)
	if err != nil {
		return QRInfo{}, err
	}
	return QRInfo{Text: q.Text, Side: q.Side(), Path: q.SVGPath(), MinMM: q.MinMM()}, nil
}

// AddQRSticker — код ссылки в библиотеку («Мои») и стикером на страницу page листа, как щелчок
// по плитке: один шаг отмены, плитка — в недавние. Запись библиотеки после отмены остаётся.
func (a *App) AddQRSticker(spreadID string, page int, link string) (r StickerResult, err error) {
	defer a.guardEdit("AddQRSticker", &r.View, &err)
	s, err := a.decorStore()
	if err != nil {
		return StickerResult{View: a.editor().View()}, err
	}
	it, err := s.AddQR(link)
	if err != nil {
		return StickerResult{View: a.editor().View()}, err
	}
	log.Printf("QR-код: ссылка %d знаков", utf8.RuneCountInString(link))
	a.relinkLib(s)
	return a.AddSticker(spreadID, page, it.ID)
}
