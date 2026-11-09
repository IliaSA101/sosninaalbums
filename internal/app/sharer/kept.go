package sharer

// Сохранённые страницы «Продолжить» (159): оборванная выгрузка не выбрасывает уже нарисованные
// страницы, а оставляет их в <кэш>/share/kp-* до конца сессии. Слот один; в нём фото клиента, поэтому
// он живёт ровно до события из спеки: успех, «Отозвать», новый проект на пути, выход, другой ключ.

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/IliaSA101/sosninaalbums/api/albums"
	"github.com/IliaSA101/sosninaalbums/internal/export"
	"github.com/IliaSA101/sosninaalbums/internal/share"
)

const keptPrefix = "kp-"

// kept — страницы оборванной выгрузки. Ключ — путь, отпечаток, хэш сцены и поколение шрифтов:
// другой ключ — страницы устарели.
type kept struct {
	path, uid, scene string
	fonts            uint64
	dir              string           // <кэш>/share/kp-<случайное>, 0700
	pages            []export.WebPage // как вернул RunWeb
	man              *albums.Manifest // манифест первой подготовки: эталон против подмены между попытками
}

// isKept — папка хода уже сохранённая (повтор шёл по ней).
func isKept(dir string) bool { return strings.HasPrefix(filepath.Base(dir), keptPrefix) }

// settle — конец хода: успех убирает всё, обрыв сохраняет страницы для «Продолжить». До s.end:
// иначе быстрый «Продолжить» слота не увидел бы.
func (s *Service) settle(j *job, err error) {
	if err == nil {
		s.dropKept()
		removeDir(j.dir)
		return
	}
	s.keep(j)
}

// keep кладёт страницы хода в слот (папка up-* переименовывается в kp-*, прежний слот удаляется).
// Выход программы и новый проект на пути (run.stop), а также ход, не дошедший до манифеста, — только
// удаление: слот под них не нужен, а страницы чужого альбома держать нельзя.
func (s *Service) keep(j *job) {
	var rm []string
	s.mu.Lock()
	switch {
	case j.man == nil: // страницы не готовы целиком
		rm = append(rm, j.dir)
	case s.closing || j.run.stop:
		rm = append(rm, j.dir, s.takeLocked())
	case isKept(j.dir): // повтор шёл по слоту: он на месте, если его не сбросили (Revoke, OnSaved)
		if s.kp == nil || s.kp.dir != j.dir {
			rm = append(rm, j.dir)
		}
	default:
		dst := filepath.Join(filepath.Dir(j.dir), keptPrefix+strings.TrimPrefix(filepath.Base(j.dir), tempPrefix))
		if err := os.Rename(j.dir, dst); err != nil {
			log.Printf("выгрузка: страницы не сохранены для продолжения")
			rm = append(rm, j.dir)
			break
		}
		rm = append(rm, s.takeLocked())
		s.kp = &kept{path: j.Path, uid: j.UID, scene: j.Scene, fonts: j.Fonts, dir: dst, pages: j.pages, man: j.man}
	}
	s.mu.Unlock()
	for _, d := range rm {
		removeDir(d)
	}
}

// takeLocked забирает слот (под mu) и отдаёт его папку на удаление; "" — слота не было.
func (s *Service) takeLocked() string {
	if s.kp == nil {
		return ""
	}
	dir := s.kp.dir
	s.kp = nil
	return dir
}

// dropKept удаляет слот.
func (s *Service) dropKept() {
	s.mu.Lock()
	dir := s.takeLocked()
	s.mu.Unlock()
	removeDir(dir)
}

// dropKeptFor удаляет слот альбома path (Revoke, новый проект на пути); слот другого пути остаётся.
func (s *Service) dropKeptFor(path string) {
	s.mu.Lock()
	var dir string
	if s.kp != nil && path != "" && s.h.Fold(s.kp.path) == s.h.Fold(path) {
		dir = s.takeLocked()
	}
	s.mu.Unlock()
	removeDir(dir)
}

// Shutdown — выход из программы: слот удаляется сразу, а ход, который закончится позже (shutdown не
// ждёт фон), не сохранит ничего (closing в keep). Зовёт App.shutdown до остановки фона.
func (s *Service) Shutdown() {
	s.mu.Lock()
	s.closing = true
	dir := s.takeLocked()
	s.mu.Unlock()
	removeDir(dir)
}

// match отдаёт слот, если он этого хода; чужой слот (другой альбом, сцена, шрифты) удаляется сразу:
// фото клиента без нужды не держим.
func (s *Service) match(j *job) *kept {
	s.mu.Lock()
	k := s.kp
	if k == nil {
		s.mu.Unlock()
		return nil
	}
	if s.h.Fold(k.path) == s.h.Fold(j.Path) && k.uid == j.UID && k.scene == j.Scene && k.fonts == j.Fonts {
		s.mu.Unlock()
		return k
	}
	dir := s.takeLocked()
	s.mu.Unlock()
	removeDir(dir)
	return nil
}

// reuse берёт страницы из слота и собирает манифест заново: он обязан совпасть с эталоном первой
// подготовки (страницы, а не название и срок). Не совпало, файла нет, не читается — слот удаляется,
// строка в log, ok=false: вызывающий готовит заново. Остановка — ошибка (слот остаётся).
func (s *Service) reuse(ctx context.Context, j *job, gen string, lim albums.Limits) (m *albums.Manifest, ok bool, err error) {
	k := s.match(j)
	if k == nil {
		return nil, false, nil
	}
	m, err = share.BuildManifest(ctx, k.dir, k.pages, j.Cover != nil, j.Spec, j.title, gen, lim)
	if cerr := ctx.Err(); cerr != nil {
		return nil, false, cerr
	}
	if err != nil || m.Book != k.man.Book || !slices.Equal(m.Pages, k.man.Pages) {
		log.Printf("ссылка: сохранённые страницы не годятся, готовлю заново")
		s.mu.Lock()
		var dir string
		if s.kp == k {
			dir = s.takeLocked()
		}
		s.mu.Unlock()
		removeDir(dir)
		return nil, false, nil
	}
	removeDir(j.dir) // пустая up-* из Share: страницы лежат в kp-*
	j.dir, j.pages, j.man = k.dir, k.pages, k.man
	return m, true, nil
}
