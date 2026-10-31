package fontlib

import (
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/paths"
	"github.com/IliaSA101/sosninaalbums/internal/text"
)

// nowMs — время записи; подменяется в тестах порядка.
var nowMs = func() int64 { return time.Now().UnixMilli() }

// rebuild строит реестр по файлам папки (нет fonts.json или он битый): берутся только
// «<16 hex>.<ttf|otf>», чей sha256 совпал с именем и кто прошёл text.Inspect. Не прошедшие и
// чужие имена не трогаются (строка в log.txt без имени). Имени исходного файла и времени загрузки
// в папке нет: время — по файлу, имя гарнитуры — из файла (пусто — «Шрифт <ключ>»). Два файла с
// одним начертанием (заменённый скрытый и новый) — остаётся более новый. memoryOnly — реестр
// не записывается (сеанс только чтение).
func (s *Store) rebuild(memoryOnly bool) {
	list, err := os.ReadDir(s.dir)
	if err != nil {
		return
	}
	var all []*entry
	for _, de := range list {
		key, ext, ok := strings.Cut(de.Name(), ".")
		if !ok || !de.Type().IsRegular() || !validKey(key) || (ext != "ttf" && ext != "otf") {
			continue
		}
		if len(all) >= MaxFonts {
			log.Printf("шрифты: в папке больше %d файлов — лишние пропущены", MaxFonts)
			break
		}
		b, err := readCapped(filepath.Join(s.dir, de.Name()), MaxFontBytes)
		if err != nil || keyOf(b) != key {
			log.Printf("шрифты: файл папки не принят (не читается или не совпал с ключом): %v", paths.NoPath(err))
			continue
		}
		info, err := text.Inspect(b)
		if err != nil || info.Ext != ext {
			log.Printf("шрифты: файл папки не принят: %v", err)
			continue
		}
		fam := oneLine(info.Family)
		if fam == "" {
			fam = "Шрифт " + key[:6]
		}
		var mod int64
		if fi, err := de.Info(); err == nil {
			mod = fi.ModTime().UnixMilli()
		}
		all = append(all, &entry{Key: key, Ext: ext, Family: fam, Bold: info.Bold, Italic: info.Italic,
			Cyr: info.Cyr, Restricted: info.Restricted, Name: fam, Bytes: int64(len(b)), Added: mod})
	}
	sort.SliceStable(all, func(i, j int) bool { // старые раньше, новые перекрывают
		if all[i].Added != all[j].Added {
			return all[i].Added < all[j].Added
		}
		return all[i].Key < all[j].Key
	})
	latest := map[string]int{}
	for i, e := range all {
		latest[faceID(e)] = i
	}
	for i, e := range all {
		if latest[faceID(e)] == i {
			s.entries = append(s.entries, e)
		}
	}
	if len(s.entries) > 0 && !memoryOnly {
		if err := s.save(); err != nil {
			log.Printf("шрифты: реестр не записан: %v", paths.NoPath(err))
		}
	}
}

// faceID — гарнитура и начертание: «один файл на начертание».
func faceID(e *entry) string {
	return text.FamilyID(e.Family) + string(rune('0'+styleRank(e.Bold, e.Italic)))
}
