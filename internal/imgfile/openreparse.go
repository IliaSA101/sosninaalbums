package imgfile

import (
	"fmt"
	"io/fs"
	"os"
	"runtime"

	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// OpenNoReparse — открыть обычный файл, не разворачивая ссылку в последнем звене (на Windows; детали — openNoReparse
// в openreparse_windows.go). Сначала paths.CheckLocalPath, защита в глубину: пустое, относительное и нелокальное
// (UNC, устройство) имя отказывает здесь, а не в ОС — вызывающий мог забыть проверку.
func OpenNoReparse(path string) (*os.File, error) {
	if err := paths.CheckLocalPath(path); err != nil {
		return nil, fmt.Errorf("открытие файла: %w", err)
	}
	return openNoReparse(path)
}

// MaybeRegular — по сведениям о пути запись может оказаться обычным файлом. Кроме обычного, на Windows это
// ModeIrregular: так Go называет облачную заглушку (OneDrive «только в сети»), но так же и точку соединения,
// и другие точки повторной обработки — различает их только дескриптор (OpenNoReparse, OpenRegular), там и решение.
// Папка и ссылка сюда не попадают (у них свои биты режима).
func MaybeRegular(m fs.FileMode) bool {
	return m.IsRegular() || runtime.GOOS == "windows" && m.Type() == fs.ModeIrregular
}

// cloudStubAt — по пути лежит облачная заглушка (OS-часть: openreparse_windows.go / openreparse_other.go);
// переменная — шов для теста логики предиката на любой ОС.
var cloudStubAt = cloudStubPath

// RegularOrCloud — запись path, сведения о которой st (Stat, Lstat, StatLocal, DirEntry.Info), — обычный файл или
// облачная заглушка (OneDrive «только в сети»; на Windows Go называет её ModeIrregular). Единый предикат «файл
// на месте» для путей, где раньше стояло st.Mode().IsRegular(): иначе заглушка считалась пропавшей. Тег читается
// по пути без скачивания данных (доступ только к атрибутам); окончательное решение при открытии за дескриптором
// (OpenNoReparse, OpenRegular). Точка соединения, AppExecLink и прочее — не файл.
func RegularOrCloud(path string, st fs.FileInfo) bool {
	if st == nil {
		return false
	}
	m := st.Mode()
	return m.IsRegular() || m.Type() == fs.ModeIrregular && cloudStubAt(path)
}

// Атрибуты файла, данные которого не на диске (OneDrive и подобные): чтение скачает их из облака.
const (
	attrOffline      = 0x1000
	attrRecallOpen   = 0x40000
	attrRecallAccess = 0x400000
)

// Dehydrated — по сведениям st данные файла не на диске: открытие или чтение скачает их из облака. Сканер не читает
// у таких файлов заголовок (иначе лента папки OneDrive скачала бы её целиком — решение 160-M1 в decisions 0.37).
// Вне Windows всегда false.
func Dehydrated(st fs.FileInfo) bool {
	if st == nil {
		return false
	}
	return attrsDehydrated(fileAttrs(st))
}

// attrsDehydrated — атрибуты Windows говорят, что данные файла в облаке.
func attrsDehydrated(attrs uint32) bool {
	return attrs&(attrOffline|attrRecallOpen|attrRecallAccess) != 0
}
