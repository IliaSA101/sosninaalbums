package guidevideo

import (
	"cmp"
	"slices"
	"strconv"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/decor/fetch"
)

// choice — что взять для id: самая новая версия, годная программе, и вид файла.
type choice struct {
	video fetch.Video
	kind  string
	file  fetch.VideoFile
}

// pick — для каждого id самая новая версия с minApp не выше app (у старой программы свой ролик,
// 140-17). Вид: darwin — mp4, иначе webm; нужного нет — другой (0.30-140-3).
func pick(cat fetch.Catalog, app, goos string) map[string]choice {
	order := [2]string{fetch.KindWebM, fetch.KindMP4}
	if goos == "darwin" {
		order = [2]string{fetch.KindMP4, fetch.KindWebM}
	}
	out := map[string]choice{}
	for _, v := range cat.Videos {
		if v.NeedsApp(app) || !fetch.ValidID(v.ID) {
			continue
		}
		if old, ok := out[v.ID]; ok && old.video.Version >= v.Version {
			continue
		}
		for _, kind := range order {
			if f, ok := v.Files[kind]; ok {
				out[v.ID] = choice{video: v, kind: kind, file: f}
				break
			}
		}
	}
	return out
}

// ids — id выбранных роликов по алфавиту.
func ids(m map[string]choice) []string {
	keys := make([]string, 0, len(m))
	for id := range m {
		keys = append(keys, id)
	}
	slices.SortFunc(keys, cmp.Compare[string])
	return keys
}

const maxVersion = 999999

// fileName — имя файла в кэше: «<id>-<версия>.<вид>». Состоит только из проверенных частей, адрес
// каталога в него не входит; «-» делает имя не именем устройства Windows, длина ≤ 52.
func fileName(id string, ver int, kind string) (string, bool) {
	if !fetch.ValidID(id) || ver < 1 || ver > maxVersion || kind != fetch.KindWebM && kind != fetch.KindMP4 {
		return "", false
	}
	return id + "-" + strconv.Itoa(ver) + "." + kind, true
}

// parseName — обратный разбор имени: id — до последнего «-», число каноничное, вид из двух.
// Чистка старых версий трогает только ok-имена своего id (id статей бывают префиксами друг друга:
// cover и cover-export, security Н1).
func parseName(name string) (id string, ver int, kind string, ok bool) {
	base, kind, found := strings.Cut(name, ".")
	if !found || kind != fetch.KindWebM && kind != fetch.KindMP4 {
		return "", 0, "", false
	}
	i := strings.LastIndexByte(base, '-')
	if i < 0 {
		return "", 0, "", false
	}
	id, num := base[:i], base[i+1:]
	ver, err := strconv.Atoi(num)
	if err != nil || strconv.Itoa(ver) != num || ver < 1 || ver > maxVersion || !fetch.ValidID(id) {
		return "", 0, "", false
	}
	return id, ver, kind, true
}

// validSum — sha256 в нижнем регистре, 64 знака (как fetch.validSum).
func validSum(s string) bool {
	return len(s) == 64 && strings.Trim(s, "0123456789abcdef") == ""
}
