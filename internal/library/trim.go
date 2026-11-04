package library

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Пределы кэша миниатюр: сверх CacheMax чистим до CacheTarget.
const (
	CacheMax    int64 = 2 << 30
	CacheTarget int64 = 3 << 29 // 1.5 ГБ
)

// tmpMaxAge — .tmp старше этого срока считаются остатком оборванной записи.
const tmpMaxAge = time.Hour

// Trim удаляет самые старые (по mtime) файлы кэша, если его размер больше limit,
// пока он не станет не больше target. Свежие записи (*.tmp) не трогает,
// а старше часа (остаток после сбоя) удаляет.
// Безопасно вызывать в фоне: отдача миниатюр не блокируется, а потерянная
// копия просто сделается заново.
func (c *Cache) Trim(limit, target int64) {
	if !c.trimMu.TryLock() {
		return
	}
	defer c.trimMu.Unlock()
	type item struct {
		path string
		size int64
		mod  int64
	}
	var files []item
	var total int64
	_ = filepath.WalkDir(c.dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if strings.HasSuffix(p, ".tmp") {
			if info, err := d.Info(); err == nil && time.Since(info.ModTime()) > tmpMaxAge {
				_ = os.Remove(p) //nolint:gosec // обходим собственный кэш; символьных ссылок в нём нет; не вышло сейчас — уберёт следующий Trim
			}
			return nil
		}
		if info, err := d.Info(); err == nil {
			files = append(files, item{p, info.Size(), info.ModTime().UnixNano()})
			total += info.Size()
		}
		return nil
	})
	if total <= limit {
		return
	}
	sort.Slice(files, func(i, j int) bool { return files[i].mod < files[j].mod })
	for _, f := range files {
		if total <= target {
			return
		}
		if os.Remove(f.path) == nil {
			total -= f.size
		}
	}
}
