package app

import (
	"context"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/app/facefit"
	"github.com/IliaSA101/sosninaalbums/internal/library"
)

// Подгон кадров по лицам (спека 130) и силуэт под лицо: тонкие обёртки над internal/app/facefit.
// Здесь только то, что принадлежит App: guardEdit для биндингов и Host службы.

// FacesResult — ответ FitFaces: вид после правки (при Report.Asked правки нет — вид прежний) и отчёт.
type FacesResult struct {
	View   model.View       `json:"view"`
	Report model.FaceReport `json:"report"`
}

// FitFaces — «Подогнать по лицам» на листе spreadID (виньетка): кадры фото ставятся по лицам,
// одна правка — один шаг отмены. ref — место-образец «Остальные — как это лицо» ("" — норма листа
// из памяти или начальная 0,35 / 0,32). mode: "" — спросить про правленые кадры (Report.Asked,
// правки нет), "keep" — оставить их, "all" — подогнать и их. Ход — событие faces:progress
// (done, total). Ошибки: уже ищу, идёт экспорт, модель лиц не загрузилась, нет листа, образец
// негоден (правки нет). Тело — facefit.Fitter.Fit.
func (a *App) FitFaces(spreadID, ref, mode string) (res FacesResult, err error) {
	defer a.guardEdit("FitFaces", &res.View, &err)
	v, rep, err := a.faceFit.Fit(spreadID, ref, mode)
	if err != nil {
		return res, err
	}
	return FacesResult{View: v, Report: rep}, nil
}

// FaceGuide — силуэт под лицо для места с фото photoID на листе spreadID (silhouette.md): окно
// места и контур в мм, норма — листа из памяти («Остальные — как это лицо») или начальная. Лица
// не ищет и проект не правит: чтение, не шаг отмены. On = false без ошибки — силуэта нет (лист
// не виньетка, места нет, групповое, «Целиком»); это украшение, а не команда. Тело — facefit.Fitter.Guide.
func (a *App) FaceGuide(spreadID, photoID string) model.FaceGuide {
	return a.faceFit.Guide(spreadID, photoID)
}

// faceHost — что подгону по лицам нужно от App. Всё, что тесты подменяют после NewApp (emit,
// stopCtx), читается в момент вызова, а не при сборке.
func (a *App) faceHost() facefit.Host {
	return facefit.Host{
		Editor: a.editor,
		Photo:  a.mediaPhotoCloud,
		// Превью своего размера: поиск лиц — расчёт с памятью, чужой размер не годится (160-M1).
		Copy: func(ctx context.Context, id string, p library.Photo, cloud bool) (string, error) {
			return a.photoCopyCtx(ctx, id, p, cloud, true, false)
		},
		Ctx:        func() context.Context { return a.stopCtx },
		Go:         a.goBG,
		ExportBusy: func() bool { running, _ := a.exp.Busy(); return running },
		Emit:       func(name string, args ...any) { a.emit(name, args...) },
		Changed:    a.changed,
		Rare:       logRare,
	}
}
