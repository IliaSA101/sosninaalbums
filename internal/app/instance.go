package app

// Замок второго экземпляра и папки данных: вынесено из app.go; поведение прежнее.

import (
	"path/filepath"

	"github.com/IliaSA101/sosninaalbums/internal/osx"
)

// Стабильный идентификатор приложения для блокировки второго экземпляра. Один во всех
// версиях: иначе 0.6 и 0.7 запустятся вместе и поделят config.json и черновик
// (межпроцессного замка нет сознательно, спека 019 §C). Сторож — TestInstanceIDStable.
const instanceID = "6f1c2b7e-3a4d-4e58-9b0a-5d2e8c7f1a93"

// instanceKey — замок второго экземпляра: у стенда QA свой, чтобы 2–3 копии wails dev
// открылись рядом (папки данных у них тоже свои, см. dataDirs).
func instanceKey(stand string) string {
	if stand == "" {
		return instanceID
	}
	return instanceID + "-stand-" + stand
}

// dataDirs — папка настроек (config.json, templates.json, Библиотека, черновик, log.txt) и кэша
// миниатюр; name — osx.DataName.
//
// Только абсолютные (osx.ConfigDir/CacheDir): без HOME, XDG_* или APPDATA/LOCALAPPDATA Join с
// пустой базой дал бы путь от текущей папки процесса. "" — папки нет
// (настройки не пишутся, миниатюры отключены), как у guideCacheDir и update.Dir.
func dataDirs(name string) (cfg, thumbs string) {
	if c := osx.ConfigDir(); c != "" {
		cfg = filepath.Join(c, name)
	}
	if k := osx.CacheDir(); k != "" {
		thumbs = filepath.Join(k, name, "thumbs")
	}
	return cfg, thumbs
}
