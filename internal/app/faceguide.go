package app

import (
	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/geom"
)

// FaceGuide — силуэт под лицо для места с фото photoID на листе spreadID (silhouette.md): окно
// места и контур в мм, норма — листа из памяти («Остальные — как это лицо») или начальная. Лица
// не ищет и проект не правит: чтение, не шаг отмены. On = false без ошибки — силуэта нет (лист
// не виньетка, места нет, групповое, «Целиком»); это украшение, а не команда.
func (a *App) FaceGuide(spreadID, photoID string) model.FaceGuide {
	ed := a.editor()
	w, h, ok := ed.FaceWindow(spreadID, photoID)
	if !ok {
		return noGuide()
	}
	b := geom.FaceBust(w, h, album.BaseFaceNorm(a.faceSt.normOf(ed, spreadID)))
	if len(b.Path) == 0 {
		return noGuide()
	}
	return model.FaceGuide{On: true, W: w, H: h, EyeY: b.EyeY, Path: b.Path}
}

// noGuide — «силуэта нет»: путь пустой список, а не null, чтобы интерфейс не споткнулся.
func noGuide() model.FaceGuide { return model.FaceGuide{Path: [][2]float64{}} }
