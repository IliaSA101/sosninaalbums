package osx

// VolumeOf — «какой это диск» для привязки места агента (153, design §4.2): стабильный номер тома,
// не буква и не точка монтирования. Буква меняется при пересадке флешки, номер — нет. Системные
// вызовы по ОС: volid_windows.go, volid_linux.go (+ volid_mounts.go), volid_other.go.

import (
	"errors"
	"strings"
	"unicode/utf8"
)

// Пределы: данные идут в config.json и в сравнение с ним, чужого размера не берём.
const (
	maxVolID    = 256
	maxVolRel   = 1024
	maxVolPath  = 32767 // длиннейший путь Windows
	maxVolEntry = 4096  // записей by-uuid / by-label за один проход
)

// Vol — том, на котором лежит путь.
type Vol struct {
	Root  string // текущий корень: «E:\» или точка монтирования; меняется между запусками
	ID    string // «win:<серийный номер hex8>:<ФС>» или «uuid:<UUID ФС>»; не меняется
	Label string // метка тома — только для показа (может быть пустой)
	Rel   string // путь внутри тома от корня ФС, через «/», без начального «/»; пусто — корень
}

var (
	// ErrVolGone — тома с таким ID сейчас нет (диск вынут, картридер пуст).
	ErrVolGone = errors.New("тома с таким номером нет")
	// errNoVolID — у тома нет стабильного номера (tmpfs, overlay, ZFS, сеть, CD-ROM, сетевой диск).
	errNoVolID = errors.New("у тома нет стабильного номера")
)

// VolumeOf — том настоящего (уже разрешённого, абсолютного) пути. Ошибка: путь негоден, тома нет
// или у него нет номера. Папка может не существовать — важен только том. Вызов может ждать
// спящий диск: вызывающий держит его под предельным временем.
func VolumeOf(path string) (Vol, error) {
	if err := checkVolPath(path); err != nil {
		return Vol{}, err
	}
	v, err := volumeOS(path)
	if err != nil {
		return Vol{}, err
	}
	if !validVolID(v.ID) || !validVolRel(v.Rel) {
		return Vol{}, errNoVolID
	}
	return v, nil
}

// VolumePath — где сейчас лежит rel на томе id: корень найденного тома + rel (в формате ОС). Тома
// нет — ErrVolGone; label — его текущая метка. Для показа списка мест (153 §4.5).
func VolumePath(id, rel string) (path, label string, err error) {
	if !validVolID(id) || !validVolRel(rel) {
		return "", "", errNoVolID
	}
	return volumePathOS(id, rel)
}

func checkVolPath(path string) error {
	switch {
	case path == "" || len(path) > maxVolPath:
		return errors.New("путь пуст или слишком длинный")
	case strings.IndexByte(path, 0) >= 0:
		return errors.New("в пути нулевой байт")
	case !isAbsVol(path):
		return errors.New("путь не абсолютный")
	}
	return nil
}

// validVolID — ID читаем и невелик: UTF-8 без управляющих знаков.
func validVolID(id string) bool {
	if id == "" || len(id) > maxVolID || !utf8.ValidString(id) {
		return false
	}
	for _, r := range id {
		if r < ' ' || r == 0x7f {
			return false
		}
	}
	return true
}

// validVolRel — путь внутри тома: без «..», начального «/» и нулевых байт, в пределе.
func validVolRel(rel string) bool {
	if len(rel) > maxVolRel || !utf8.ValidString(rel) || strings.IndexByte(rel, 0) >= 0 || strings.HasPrefix(rel, "/") {
		return false
	}
	for _, seg := range strings.Split(rel, "/") {
		if seg == ".." {
			return false
		}
	}
	return true
}

// cleanLabel — метка для показа: без управляющих знаков и не длиннее ID.
func cleanLabel(s string) string {
	s = strings.ToValidUTF8(s, "")
	s = strings.Map(func(r rune) rune {
		if r < ' ' || r == 0x7f {
			return -1
		}
		return r
	}, s)
	if len(s) > maxVolID {
		s = strings.ToValidUTF8(s[:maxVolID], "")
	}
	return s
}
