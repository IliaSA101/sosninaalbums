package osx

// Облачные папки по настройкам клиентов (160, design §4): разбор файлов Dropbox и Яндекс.Диска и общая очистка найденных
// корней. Без тегов сборки — тесты идут на любой ОС. Файлы настроек читает чужая программа, а подделать их может кто
// угодно с правами пользователя: предел размера до разбора, пути проходят фильтр вызывающего (CheckLocalPath), а
// подделка может лишь добавить корень — то есть запретить место (тихо там становится нельзя), но не разрешить.

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"unicode/utf8"
)

// Пределы: файл настроек клиента — единицы килобайт; больше — чужое или битое.
const (
	maxCloudFile  = 64 << 10 // читаем не больше (info.json Dropbox — сотни байт)
	maxCloudRoots = 32       // корней всего: чужой реестр или файл не раздувает список
	maxCloudPath  = 4096     // знаков в одном пути
)

// dropboxPath — {"path": "..."} в info.json.
type dropboxPath struct {
	Path string `json:"path"`
}

// parseDropboxInfo — корни из info.json Dropbox: personal.path и business.path. Битый JSON — пусто.
func parseDropboxInfo(data []byte) []string {
	var info struct {
		Personal dropboxPath `json:"personal"`
		Business dropboxPath `json:"business"`
	}
	if len(data) > maxCloudFile || json.Unmarshal(data, &info) != nil {
		return nil
	}
	var out []string
	for _, p := range []string{info.Personal.Path, info.Business.Path} {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// parseYandexConfig — корень из config.cfg Яндекс.Диска Linux: строка `dir="…"` (кавычки или без них). Остальные ключи
// (auth, proxy, exclude-dirs) не читаем.
func parseYandexConfig(data []byte) []string {
	if len(data) > maxCloudFile {
		return nil
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		key, val, ok := strings.Cut(line, "=")
		if !ok || strings.TrimSpace(key) != "dir" {
			continue
		}
		val = strings.TrimSpace(val)
		if len(val) >= 2 && val[0] == '"' && val[len(val)-1] == '"' {
			val = val[1 : len(val)-1]
		}
		if val != "" {
			return []string{val}
		}
	}
	return nil
}

// dropboxFiles — где искать info.json: Windows — %APPDATA% и %LOCALAPPDATA% (файл лежит в одном из них, зависит от
// версии клиента), остальные ОС — ~/.dropbox. Пути не проверены: фильтр вызывающего и Lstat — при чтении.
func dropboxFiles(goos, home string, getenv func(string) string) []string {
	if goos == "windows" {
		var out []string
		for _, k := range []string{"APPDATA", "LOCALAPPDATA"} {
			if v := getenv(k); v != "" {
				out = append(out, filepath.Join(v, "Dropbox", "info.json"))
			}
		}
		return out
	}
	if home == "" {
		return nil
	}
	return []string{filepath.Join(home, ".dropbox", "info.json")}
}

// readCloudFile — содержимое файла настроек: путь прошёл фильтр local, это обычный файл (не ссылка, не FIFO) не больше
// maxCloudFile, при открытии он тот же, что при Lstat. Любая неудача — false без шума: у человека клиента может не быть.
func readCloudFile(path string, local func(string) error) ([]byte, bool) {
	if !filepath.IsAbs(path) || local(path) != nil {
		return nil, false
	}
	lst, err := os.Lstat(path)
	if err != nil || !lst.Mode().IsRegular() || lst.Size() > maxCloudFile {
		return nil, false
	}
	f, err := openCloudFile(path)
	if err != nil {
		return nil, false
	}
	defer func() { _ = f.Close() }()
	if st, err := f.Stat(); err != nil || !st.Mode().IsRegular() || !os.SameFile(st, lst) {
		return nil, false // подмена между Lstat и открытием
	}
	data, err := io.ReadAll(io.LimitReader(f, maxCloudFile+1))
	if err != nil || len(data) > maxCloudFile {
		return nil, false
	}
	return data, true
}

// cleanCloudRoots — корни из настроек: абсолютные, чистые, годный UTF-8 без управляющих знаков, прошедшие фильтр local;
// без повторов (на Windows — без учёта регистра), не больше maxCloudRoots. Негодный корень пропускается.
func cleanCloudRoots(raw []string, local func(string) error) []string {
	var out []string
	for _, p := range raw {
		if len(out) == maxCloudRoots {
			break
		}
		p = strings.TrimSpace(p)
		if p == "" || len(p) > maxCloudPath || !utf8.ValidString(p) || !filepath.IsAbs(p) {
			continue
		}
		if strings.IndexFunc(p, func(r rune) bool { return r < 0x20 || r == 0x7f }) >= 0 {
			continue
		}
		if local(p) != nil { // до Clean: он превращает «//srv/share» в «/srv/share»
			continue
		}
		p = filepath.Clean(p)
		if local(p) != nil || dupRoot(out, p) {
			continue
		}
		out = append(out, p)
	}
	return out
}

func dupRoot(have []string, p string) bool {
	for _, h := range have {
		if h == p || runtime.GOOS == "windows" && strings.EqualFold(h, p) {
			return true
		}
	}
	return false
}

// Метки томов виртуальных облачных дисков Windows (160, design §4): Google Drive, Яндекс.Диск, pCloud Drive, Box.
// Сравнение по началу метки без регистра («Google Drive (G:)» бывает).
var vcloudLabels = []string{"google drive", "яндекс.диск", "yandex.disk", "pcloud", "box drive"}

// virtualCloudVol — признак виртуального облачного диска по метке и ФС тома: метка из списка, либо несъёмный диск с ФС вне
// NTFS, ReFS, FAT32, exFAT (потоковые диски отдают свою). Съёмный диск судится только по метке: на карте бывает и FAT.
func virtualCloudVol(label, fsName string, fixed bool) bool {
	l := strings.ToLower(strings.TrimSpace(label))
	if l == "box" {
		return true
	}
	for _, k := range vcloudLabels {
		if strings.HasPrefix(l, k) {
			return true
		}
	}
	if !fixed {
		return false
	}
	switch strings.ToUpper(strings.TrimSpace(fsName)) {
	case "NTFS", "REFS", "FAT32", "EXFAT":
		return false
	}
	return true
}
