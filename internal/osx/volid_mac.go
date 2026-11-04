package osx

// Номер тома на macOS (160 Т4, design §5): разбор без системных вызовов. Сами вызовы — в
// volid_darwin_fs.go (statfs, getfsstat) и volid_darwin.go (getattrlist через cgo); здесь только
// решения по их ответам, поэтому тесты идут на любой ОС, а ответ getattrlist разбирается как чужие данные.

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"runtime"
	"strings"
)

// Пределы и флаги macOS.
const (
	macVolumes    = "/Volumes/"
	maxMacMounts  = 512  // записей getfsstat за проход
	maxMacUp      = 64   // шагов вверх от несуществующей папки к существующей
	maxVolAttrBuf = 2048 // ответ getattrlist: метка до 255 знаков в UTF-8 с запасом
	mntRootFS     = 0x4000
	mntQuarantine = 0x400 // том из помеченного карантином образа (.dmg из сети): не привязываем
	mntNoWait     = 2     // MNT_NOWAIT: getfsstat не будит спящие диски
)

// macStat — одна запись statfs/getfsstat: то, что решает, годен ли том для привязки.
type macStat struct {
	Point  string // f_mntonname: точка монтирования
	Fstype string // f_fstypename
	Flags  uint32 // f_flags
}

// volAttr — ответ getattrlist по тому: UUID тома и его метка.
type volAttr struct {
	UUID string // «xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx», не нулевой
	Name string
}

// macVolPoint — точка вида «/Volumes/<X>»: ровно один элемент под /Volumes, без «.», «..» и повторных «/».
func macVolPoint(point string) bool {
	rest, ok := strings.CutPrefix(point, macVolumes)
	return ok && rest != "" && rest != "." && rest != ".." && !strings.Contains(rest, "/") && !strings.ContainsRune(rest, 0)
}

// macBindable — том можно привязать: локальный, не загрузочный, не из карантинного образа, смонтирован как /Volumes/<X>.
// Загрузочный том и Data (/Users) под /Volumes не лежат: привязка там ничего не даёт (0.36-160-7).
func macBindable(st macStat) bool {
	return st.Flags&(mntRootFS|mntQuarantine) == 0 && !darwinRemote(st.Flags, st.Fstype) && macVolPoint(st.Point)
}

// CanBind — у места по этому пути может быть привязка к тому (160, AgentPlace.Bindable): Windows и
// Linux — всегда (как в 0.35), macOS — только под /Volumes/<X>. Чистая строка, диск не трогает.
func CanBind(p string) bool { return canBind(runtime.GOOS, p) }

func canBind(goos, p string) bool {
	switch goos {
	case "windows", "linux":
		return true
	case "darwin":
		rest, ok := cutPrefixFold(path.Clean(p), macVolumes) // без Clean «/Volumes/X/../../Users» прошёл бы
		if !ok || rest == "" || strings.HasPrefix(rest, "/") {
			return false
		}
		first, _, _ := strings.Cut(rest, "/")
		return first != "." && first != ".." && !strings.ContainsRune(rest, 0)
	}
	return false
}

// cutPrefixFold — CutPrefix без учёта регистра: том APFS по умолчанию нечувствителен к нему.
func cutPrefixFold(s, prefix string) (string, bool) {
	if len(s) < len(prefix) || !strings.EqualFold(s[:len(prefix)], prefix) {
		return s, false
	}
	return s[len(prefix):], true
}

// macRel — путь p внутри точки: «» для самой точки, иначе элементы через «/». Регистр точки
// из statfs может отличаться от написанного в p (APFS без учёта регистра).
func macRel(p, point string) (string, bool) {
	rest, ok := cutPrefixFold(p, point)
	if !ok {
		return "", false
	}
	if rest == "" {
		return "", true
	}
	return strings.CutPrefix(rest, "/")
}

// macStatUp — statfs ближайшего существующего предка p: папка места может ещё не существовать,
// а важен только том (как на Linux).
func macStatUp(p string, stat func(string) (macStat, error)) (macStat, error) {
	for i := 0; i < maxMacUp; i++ {
		st, err := stat(p)
		if err == nil {
			return st, nil
		}
		parent := path.Dir(p)
		if !errors.Is(err, fs.ErrNotExist) || parent == p {
			return macStat{}, err
		}
		p = parent
	}
	return macStat{}, errors.New("слишком глубокий несуществующий путь")
}

// macVolume — том пути p по statfs его точки и чтению атрибутов. Не годный для привязки том
// (загрузочный, сетевой, вне /Volumes), нулевой UUID — errNoVolID.
func macVolume(p string, st macStat, read func(point string) (volAttr, error)) (Vol, error) {
	p = path.Clean(p)
	if !macBindable(st) {
		return Vol{}, errNoVolID
	}
	rel, ok := macRel(p, st.Point)
	if !ok {
		return Vol{}, errNoVolID
	}
	a, err := read(st.Point)
	if err != nil {
		return Vol{}, fmt.Errorf("%w: %v", errNoVolID, err)
	}
	if !macUUIDStable(st.Fstype, a.UUID) {
		return Vol{}, fmt.Errorf("%w: случайный UUID у %s", errNoVolID, st.Fstype)
	}
	return Vol{Root: st.Point, ID: "uuid:" + a.UUID, Label: macLabel(a.Name, st.Point), Rel: rel}, nil
}

// macFatFS — ФС, чей UUID macOS выводит из серийного номера тома (name-based, версия 3).
var macFatFS = map[string]bool{"msdos": true, "exfat": true}

// macUUIDStable — годен ли UUID для опознания тома. У FAT/exFAT он должен быть версии 3 (MD5 от
// серийного номера, как у msdosfs.kext): иногда getattrlist после attach отдаёт случайный версии 4
// (160-mac-N+1), и такой номер при следующем подключении другой — «Диск снова доступен» ошибся бы.
// Как на Windows и Linux, где номер — серийный номер тома FAT: нет постоянного — нет привязки.
func macUUIDStable(fstype, uuid string) bool {
	if !macFatFS[fstype] {
		return true
	}
	return len(uuid) > 14 && uuid[14] != '4'
}

// macLabel — метка тома; пустая — последний элемент точки.
func macLabel(name, point string) string {
	if l := cleanLabel(name); l != "" {
		return l
	}
	return cleanLabel(path.Base(point))
}

// macPathFromMounts — путь rel на томе id по списку смонтированных: UUID читается у каждого
// годного тома. Нет такого — ErrVolGone; два тома с одним UUID (клон диска) — тоже, путь не угадываем.
func macPathFromMounts(ms []macStat, id, rel string, read func(point string) (volAttr, error)) (p, label string, err error) {
	var points, labels []string
	for i, st := range ms {
		if i >= maxMacMounts {
			break
		}
		if !macBindable(st) {
			continue
		}
		a, err := read(st.Point)
		if err != nil || "uuid:"+a.UUID != id || !macUUIDStable(st.Fstype, a.UUID) {
			continue
		}
		points, labels = append(points, st.Point), append(labels, macLabel(a.Name, st.Point))
	}
	if len(points) != 1 {
		return "", "", ErrVolGone
	}
	return path.Join(points[0], rel), labels[0], nil
}

// parseVolAttr — ответ getattrlist(ATTR_VOL_INFO | ATTR_VOL_NAME | ATTR_VOL_UUID): u32 длина ответа,
// attrreference метки (i32 смещение от своего начала, u32 длина с NUL), 16 байт UUID. Ответ читаем
// как чужие данные: длина и смещения проверяются, метка обрезается по NUL, нулевой UUID — отказ.
func parseVolAttr(buf []byte) (volAttr, error) {
	const fixed = 4 + 8 + 16
	bad := func(why string) (volAttr, error) { return volAttr{}, fmt.Errorf("ответ getattrlist: %s", why) }
	if len(buf) < fixed || len(buf) > maxVolAttrBuf {
		return bad("размер")
	}
	le := binary.LittleEndian // macOS: amd64 и arm64, обе little-endian
	total := int64(le.Uint32(buf[0:4]))
	if total < fixed || total > int64(len(buf)) {
		return bad("длина")
	}
	var nz byte
	for _, b := range buf[12:28] {
		nz |= b
	}
	if nz == 0 {
		return volAttr{}, errNoVolID
	}
	off, n := int64(le.Uint32(buf[4:8])), int64(le.Uint32(buf[8:12])) // смещение i32: отрицательное читается как >= 2^31
	start := 4 + off
	if off >= 1<<31 || start < fixed || n > maxVolID*4 || start+n > total {
		return bad("метка вне ответа")
	}
	name := buf[start : start+n]
	if i := strings.IndexByte(string(name), 0); i >= 0 {
		name = name[:i]
	}
	u := buf[12:28]
	uuid := fmt.Sprintf("%x-%x-%x-%x-%x", u[0:4], u[4:6], u[6:8], u[8:10], u[10:16])
	return volAttr{UUID: uuid, Name: string(name)}, nil
}
