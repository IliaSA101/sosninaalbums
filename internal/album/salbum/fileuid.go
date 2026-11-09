package salbum

import (
	"encoding/json"
	"os"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// FileUID — отпечаток проекта, лежащий в файле path («Сохранить как» поверх файла с тем же
// отпечатком — тот же проект, 167-25). Файл читается и разбирается целиком (до model.MaxFileSize),
// а он — чужие данные: путь через paths.CheckLocalPath, только обычный файл (не канал и не
// устройство — Open завис бы), размер не больше model.MaxFileSize (ReadLimited). Нет файла, не читается, не проект, негодный
// отпечаток — "" (спрашивать «почему» не нужно: запись поверх будет как поверх чужого).
func FileUID(path string) string {
	if paths.CheckLocalPath(path) != nil {
		return ""
	}
	if st, err := os.Stat(path); err != nil || !st.Mode().IsRegular() {
		return ""
	}
	data, err := ReadLimited(path)
	if err != nil {
		return ""
	}
	var head struct {
		UID string `json:"uid"`
	}
	if json.Unmarshal(data, &head) != nil || !model.GoodUID(head.UID) {
		return ""
	}
	return head.UID
}
