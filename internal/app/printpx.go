package app

import "github.com/IliaSA101/sosninaalbums/internal/album/model"

// PagePxView — страница альбома по размеру файла разворота (PagePx): мм для полей формы
// «Ширина/Высота страницы». Err — строка под полем, при ней размеры нулевые.
type PagePxView struct {
	PageW float64 `json:"pageW"`
	PageH float64 `json:"pageH"`
	Err   string  `json:"err"`
}

// PagePx — «Файл разворота при 300 dpi, px» (123 §1.3): страница, при которой выгрузка даст ровно
// w × h px. Всегда 300 dpi (model.PrintDPI): выгрузка для типографии такая, а при dpi альбома 150
// ввод 7180 дал бы страницу 608 мм. Проект не меняется — поля формы ставит интерфейс, правит
// «Применить».
func (a *App) PagePx(w, h int) PagePxView {
	defer guard("PagePx")
	pw, ph, err := model.PageFromPx(w, h, model.PrintDPI)
	if err != nil {
		return PagePxView{Err: err.Error()}
	}
	return PagePxView{PageW: pw, PageH: ph}
}
