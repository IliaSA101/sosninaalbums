package app

import "github.com/IliaSA101/sosninaalbums/internal/album/model"

// WhereUsed — «где стоит» каждое фото ids для окна «Фото уже в альбоме» (098 design §2.4):
// вхождения всех записей семьи, число мест и запись единственного места (её интерфейс шлёт
// вместо ID файла для «Перенести сюда»). Только чтение: без шага отмены и рассылки вида.
func (a *App) WhereUsed(ids []string) (out []model.UsedAt) {
	defer guard("WhereUsed")
	out = []model.UsedAt{} // паника в чтении — пустой список, а не null для интерфейса
	return a.editor().WhereUsed(ids)
}
