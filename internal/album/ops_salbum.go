package album

import (
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
)

// Обёртки области salbum (023): Relink и RelinkDrive — исключение из «тело ≤ 3 операторов»,
// отметки переезжают в том же шаге через e.edit и relinkMarks (design §2).

// Refs — глубокая копия сведений обо всех фото альбома (ID → PhotoRef): App проверяет,
// какие файлы пропали, и ищет их по имени.
// Договор: только чтение под замком, проверять нечего.
func (e *Editor) Refs() (refs map[string]model.PhotoRef) {
	e.read(func(p *model.Project) { refs = salbum.Refs(p) })
	return refs
}

// Relink — перепривязка пропавших фото: moves (ID → новый путь) меняет пути,
// затем ID пересчитываются по новым путям (rekey: лента узнаёт фото как
// использованные). Папки переезжают следом: по паре «старый → новый путь» каждого
// фото paths.PrefixMap отрезает общий хвост, самая частая пара применяется к папкам
// проекта. Если после этого ни одна папка не содержит новых путей, добавляется
// addFolder (выбранная пользователем). Одна правка — один шаг отмены; свои отметки
// переезжают по тем же парам путей в том же шаге.
// Договор: до правки ничего не проверяет — salbum.Relink сам пропускает неизвестный ID,
// нелокальный путь (paths.CheckLocalPath) и чужую или нелокальную addFolder.
func (e *Editor) Relink(moves map[string]string, addFolder string) model.View {
	var files, dirs []model.PathPair
	v, _ := e.edit("", false, func(p *model.Project) error {
		files, dirs = salbum.Relink(p, moves, addFolder)
		return nil
	}, func() []MarkChange { return e.relinkMarks(files, dirs) })
	return v
}

// RelinkDrive — смена буквы диска «from → to» (одна латинская буква) для всего,
// что на from не нашлось, а на to есть: пропавшие фото и папки. Одна правка вместе
// со своими отметками.
// Договор: до правки ничего не проверяет — буквы проверяет paths.SwapDrive в salbum.RelinkDrive.
func (e *Editor) RelinkDrive(from, to string) model.View {
	var files, dirs []model.PathPair
	v, _ := e.edit("", false, func(p *model.Project) error {
		files, dirs = salbum.RelinkDrive(p, from, to)
		return nil
	}, func() []MarkChange { return e.relinkMarks(files, dirs) })
	return v
}
