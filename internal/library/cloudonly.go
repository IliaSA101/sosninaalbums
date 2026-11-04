package library

import (
	"errors"
	"io/fs"

	"github.com/IliaSA101/sosninaalbums/internal/imgfile"
)

// ErrCloudOnly — данные файла не на диске (OneDrive «только в сети»): чтение заголовка скачало бы его из облака,
// а на папке из тысяч фото — всю папку. Сканер такие файлы не открывает (решение 160-M1 в decisions 0.37): фото
// появится в ленте после «Всегда хранить на этом устройстве» и обновления папок.
var ErrCloudOnly = errors.New("файл только в облаке")

// whyCloudOnly — причина для окна «не прочитано»; без совета про Paint: файл не повреждён.
const whyCloudOnly = "файл только в облаке, на компьютере его нет. В OneDrive выберите «Всегда хранить на этом устройстве» и обновите папки"

// cloudOnlyError — ErrCloudOnly с готовой причиной (сканер берёт её без второго открытия файла).
func cloudOnlyError() error { return &unreadError{err: ErrCloudOnly, why: whyCloudOnly} }

// regularEntry — запись обхода — обычный файл или облачная заглушка (imgfile.RegularOrCloud); тип без Info() хватает
// для обычного файла, заглушку определяет тег.
func regularEntry(p string, d fs.DirEntry) bool {
	t := d.Type()
	if t.IsRegular() {
		return true
	}
	if t != fs.ModeIrregular {
		return false
	}
	info, err := d.Info()
	return err == nil && imgfile.RegularOrCloud(p, info)
}
