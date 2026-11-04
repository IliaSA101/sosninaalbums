package osx

// Тесты номера тома macOS (160 Т4) без macOS: разбор ответа getattrlist, выбор тома по statfs, поиск
// по списку томов. Настоящие образы — volid_darwin_test.go (go-mac).

import (
	"encoding/binary"
	"errors"
	"io/fs"
	"runtime"
	"strings"
	"testing"
)

// attrBuf — ответ getattrlist как его кладёт ядро: длина, attrreference (смещение 24 от своего начала), UUID, метка с NUL.
func attrBuf(uuid [16]byte, name string) []byte {
	total := 28 + len(name) + 1
	b := make([]byte, total)
	binary.LittleEndian.PutUint32(b[0:], uint32(total))
	binary.LittleEndian.PutUint32(b[4:], 24)
	binary.LittleEndian.PutUint32(b[8:], uint32(len(name)+1))
	copy(b[12:], uuid[:])
	copy(b[28:], name)
	return b
}

var testUUID = [16]byte{0xde, 0xad, 0xbe, 0xef, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11}

const testUUIDStr = "deadbeef-0001-0203-0405-060708090a0b"

func TestParseVolAttr(t *testing.T) {
	a, err := parseVolAttr(attrBuf(testUUID, "Фото 2026"))
	if err != nil || a.UUID != testUUIDStr || a.Name != "Фото 2026" {
		t.Fatalf("годный ответ: %+v, %v", a, err)
	}
	if !validUUID(a.UUID) || !validVolID("uuid:"+a.UUID) {
		t.Errorf("UUID %q не годится для ID", a.UUID)
	}
	// Запас в конце буфера (cgo отдаёт весь maxVolAttrBuf) и пустая метка.
	padded := append(attrBuf(testUUID, ""), make([]byte, maxVolAttrBuf-29)...)
	if a, err = parseVolAttr(padded); err != nil || a.UUID != testUUIDStr || a.Name != "" {
		t.Errorf("с запасом и пустой меткой: %+v, %v", a, err)
	}
	// NUL внутри метки — обрезка по нему.
	if a, err = parseVolAttr(attrBuf(testUUID, "ab\x00cd")); err != nil || a.Name != "ab" {
		t.Errorf("NUL в метке: %+v, %v", a, err)
	}
}

func TestParseVolAttrBad(t *testing.T) {
	good := func() []byte { return attrBuf(testUUID, "Диск") }
	put := func(b []byte, at int, v uint32) []byte { binary.LittleEndian.PutUint32(b[at:], v); return b }
	cases := []struct {
		name string
		buf  []byte
		want error // nil — любая ошибка
	}{
		{"nil", nil, nil},
		{"пусто", []byte{}, nil},
		{"короче фиксированной части", good()[:27], nil},
		{"длина больше буфера", put(good(), 0, 9999), nil},
		{"длина меньше фиксированной", put(good(), 0, 10), nil},
		{"нулевой UUID", attrBuf([16]byte{}, "x"), errNoVolID},
		{"смещение назад", put(good(), 4, 0xFFFFFFF0), nil},
		{"смещение в UUID", put(good(), 4, 10), nil},
		{"смещение за ответом", put(good(), 4, 5000), nil},
		{"длина метки огромная", put(good(), 8, 0xFFFFFFFF), nil},
		{"метка выходит за ответ", put(good(), 8, 100), nil},
		{"буфер больше предела", make([]byte, maxVolAttrBuf+1), nil},
	}
	for _, c := range cases {
		a, err := parseVolAttr(c.buf)
		if err == nil {
			t.Errorf("%s: ждали ошибку, получили %+v", c.name, a)
		} else if c.want != nil && !errors.Is(err, c.want) {
			t.Errorf("%s: ошибка %v, ждали %v", c.name, err, c.want)
		}
	}
	// Управляющие знаки метки уходят при показе: ID от них не зависит.
	a, err := parseVolAttr(attrBuf(testUUID, "a\x01b\xffc"))
	if err != nil || macLabel(a.Name, "/Volumes/X") != "abc" {
		t.Errorf("грязная метка: %+v → %q, %v", a, macLabel(a.Name, "/Volumes/X"), err)
	}
}

// Ответ ядра на чужих данных не паникует и не отдаёт негодный ID.
func FuzzParseVolAttr(f *testing.F) {
	f.Add(attrBuf(testUUID, "Фото"))
	f.Add(attrBuf(testUUID, ""))
	f.Add([]byte{})
	f.Add(make([]byte, 28))
	f.Add([]byte("\xff\xff\xff\xff\xf0\xff\xff\xff\xff\xff\xff\xff"))
	f.Fuzz(func(t *testing.T, b []byte) {
		a, err := parseVolAttr(b)
		if err != nil {
			return
		}
		if !validUUID(a.UUID) || len(a.UUID) != 36 || a.UUID == "00000000-0000-0000-0000-000000000000" {
			t.Fatalf("негодный UUID %q", a.UUID)
		}
		if len(a.Name) > maxVolID*4 || strings.IndexByte(a.Name, 0) >= 0 {
			t.Fatalf("негодная метка %q", a.Name)
		}
		if l := macLabel(a.Name, "/Volumes/X"); len(l) > maxVolID || strings.ContainsAny(l, "\x00\x01\x7f") {
			t.Fatalf("метка для показа %q", l)
		}
	})
}

func TestMacBindable(t *testing.T) {
	const local = 0x1000 // MNT_LOCAL
	cases := []struct {
		name string
		st   macStat
		want bool
	}{
		{"флешка exfat", macStat{"/Volumes/CARD", "exfat", local}, true},
		{"fat", macStat{"/Volumes/CARD", "msdos", local}, true},
		{"apfs", macStat{"/Volumes/Work", "apfs", local}, true},
		{"метка с пробелом", macStat{"/Volumes/CARD 1", "exfat", local}, true},
		{"загрузочный", macStat{"/", "apfs", local | mntRootFS}, false},
		{"Data", macStat{"/System/Volumes/Data", "apfs", local}, false},
		{"корень под /Volumes с ROOTFS", macStat{"/Volumes/Macintosh HD", "apfs", local | mntRootFS}, false},
		{"сеть smbfs", macStat{"/Volumes/share", "smbfs", 0}, false},
		{"nfs с MNT_LOCAL у подделки", macStat{"/Volumes/x", "nfs", local}, false},
		{"неизвестный тип", macStat{"/Volumes/x", "ntfs", local}, false},
		{"без MNT_LOCAL", macStat{"/Volumes/x", "exfat", 0}, false},
		{"образ из сети (карантин)", macStat{"/Volumes/x", "exfat", local | 0x400}, false},
		{"вложенная точка", macStat{"/Volumes/a/b", "exfat", local}, false},
		{"сам /Volumes", macStat{"/Volumes", "exfat", local}, false},
		{"/Volumes/", macStat{"/Volumes/", "exfat", local}, false},
		{"..", macStat{"/Volumes/..", "exfat", local}, false},
		{"вне /Volumes", macStat{"/mnt/x", "exfat", local}, false},
		{"пусто", macStat{}, false},
	}
	for _, c := range cases {
		if got := macBindable(c.st); got != c.want {
			t.Errorf("%s: %v, ждали %v", c.name, got, c.want)
		}
	}
}

func TestCanBind(t *testing.T) {
	for _, c := range []struct {
		goos, p string
		want    bool
	}{
		{"linux", "/home/u/Фото", true},
		{"windows", `D:\Съёмки`, true},
		{"darwin", "/Volumes/CARD", true},
		{"darwin", "/Volumes/CARD/DCIM/100", true},
		{"darwin", "/volumes/card/x", true}, // APFS без учёта регистра
		{"darwin", "/Volumes/My Card/x", true},
		{"darwin", "/Users/u/Pictures", false},
		{"darwin", "/", false},
		{"darwin", "/Volumes", false},
		{"darwin", "/Volumes/", false},
		{"darwin", "/Volumes//x", true}, // Clean сводит к /Volumes/x
		{"darwin", "/Volumes/X/../../Users/u", false},
		{"darwin", "/Volumes/X/../Y/a", true},
		// Загрузочный том под /Volumes — символическая ссылка на «/»; по строке её не отличить (нужен Lstat, а
		// CanBind диск не трогает): окно покажет «Привязать…», отказ даст VolumeOf (MNT_ROOTFS, TestMacBindable).
		{"darwin", "/Volumes/Macintosh HD/Users/u", true},
		{"darwin", "/Volumes/../etc", false},
		{"darwin", "/Volumes/./x", true},
		{"darwin", "/Volumes/x\x00y", false},
		{"darwin", "", false},
		{"darwin", "Volumes/x", false},
		{"freebsd", "/mnt/x", false},
		{"plan9", "/n/x", false},
	} {
		if got := canBind(c.goos, c.p); got != c.want {
			t.Errorf("canBind(%s, %q) = %v, ждали %v", c.goos, c.p, got, c.want)
		}
	}
	if CanBind("/Volumes/x") != canBind(runtime.GOOS, "/Volumes/x") {
		t.Error("CanBind не по ОС процесса")
	}
}

// fakeMac — поддельные тома: точка → UUID и метка; read отдаёт их, как getattrlist.
type fakeMac struct {
	uuid map[string]string
	name map[string]string
	fail map[string]bool
}

func (f fakeMac) read(point string) (volAttr, error) {
	if f.fail[point] {
		return volAttr{}, errors.New("EIO")
	}
	u, ok := f.uuid[point]
	if !ok {
		return volAttr{}, errors.New("нет тома")
	}
	return volAttr{UUID: u, Name: f.name[point]}, nil
}

const macLocal = 0x1000

func TestMacVolume(t *testing.T) {
	f := fakeMac{uuid: map[string]string{"/Volumes/CARD": testUUIDStr}, name: map[string]string{"/Volumes/CARD": "CARD"}}
	st := macStat{"/Volumes/CARD", "exfat", macLocal}
	v, err := macVolume("/Volumes/CARD/Съёмки/2026", st, f.read)
	if err != nil || v.ID != "uuid:"+testUUIDStr || v.Label != "CARD" || v.Rel != "Съёмки/2026" || v.Root != "/Volumes/CARD" {
		t.Fatalf("том: %+v, %v", v, err)
	}
	if !validVolID(v.ID) || !validVolRel(v.Rel) {
		t.Errorf("негодные ID/Rel: %+v", v)
	}
	if v, err = macVolume("/Volumes/CARD", st, f.read); err != nil || v.Rel != "" {
		t.Errorf("корень тома: %+v, %v", v, err)
	}
	// Регистр точки из statfs и написание в пути расходятся (APFS без учёта регистра).
	if v, err = macVolume("/volumes/card/DCIM", st, f.read); err != nil || v.Rel != "DCIM" {
		t.Errorf("другой регистр: %+v, %v", v, err)
	}
	// «..» и повторные «/» гасит Clean: Rel не выходит из тома.
	if v, err = macVolume("/Volumes/CARD/a/../b//c", st, f.read); err != nil || v.Rel != "b/c" {
		t.Errorf("грязный путь: %+v, %v", v, err)
	}
	// Метка пуста — последний элемент точки.
	f.name["/Volumes/CARD"] = ""
	if v, err = macVolume("/Volumes/CARD/x", st, f.read); err != nil || v.Label != "CARD" {
		t.Errorf("метка по точке: %+v, %v", v, err)
	}
	// Путь не под точкой (statfs соврал или гонка) — нет номера.
	if _, err = macVolume("/Volumes/CARDX/a", st, f.read); !errors.Is(err, errNoVolID) {
		t.Errorf("сосед по префиксу: %v", err)
	}
	// Загрузочный, Data, сеть — без номера, чтение атрибутов даже не начинается.
	boom := func(string) (volAttr, error) {
		t.Fatal("атрибуты негодного тома не читаются")
		return volAttr{}, nil
	}
	for _, s := range []macStat{{"/", "apfs", macLocal | mntRootFS}, {"/System/Volumes/Data", "apfs", macLocal}, {"/Volumes/s", "smbfs", 0}} {
		if _, err = macVolume("/Users/u/x", s, boom); !errors.Is(err, errNoVolID) {
			t.Errorf("%+v: %v", s, err)
		}
	}
	// Атрибуты не прочитались — нет номера.
	f.fail = map[string]bool{"/Volumes/CARD": true}
	if _, err = macVolume("/Volumes/CARD/x", st, f.read); !errors.Is(err, errNoVolID) {
		t.Errorf("ошибка чтения: %v", err)
	}
}

// 160-mac-N+1: у FAT/exFAT UUID версии 4 (случайный, getattrlist после attach) тома не опознаёт; у APFS/HFS версия 4 — норма.
func TestMacRandomUUIDFat(t *testing.T) {
	const v3, v4 = "10ba4531-5bb9-3730-a357-a8b98dd572c8", "7d749da2-7bd3-4aae-bccb-2bcee51d9b0f"
	for _, fs := range []string{"msdos", "exfat"} {
		f := fakeMac{uuid: map[string]string{"/Volumes/SD": v4}, name: map[string]string{"/Volumes/SD": "SD"}}
		st := macStat{"/Volumes/SD", fs, macLocal}
		if v, err := macVolume("/Volumes/SD/DCIM", st, f.read); !errors.Is(err, errNoVolID) {
			t.Errorf("%s, версия 4: %+v, %v", fs, v, err)
		}
		// Том не находится и по записанному номеру: случайный не совпадёт и с самим собой.
		if _, _, err := macPathFromMounts([]macStat{st}, "uuid:"+v4, "DCIM", f.read); !errors.Is(err, ErrVolGone) {
			t.Errorf("%s, поиск по версии 4: %v", fs, err)
		}
		f.uuid["/Volumes/SD"] = v3
		v, err := macVolume("/Volumes/SD/DCIM", st, f.read)
		if err != nil || v.ID != "uuid:"+v3 {
			t.Errorf("%s, версия 3: %+v, %v", fs, v, err)
		}
		if p, _, err := macPathFromMounts([]macStat{st}, v.ID, "DCIM", f.read); err != nil || p != "/Volumes/SD/DCIM" {
			t.Errorf("%s, поиск по версии 3: %q %v", fs, p, err)
		}
	}
	for _, fs := range []string{"apfs", "hfs"} {
		f := fakeMac{uuid: map[string]string{"/Volumes/X": v4}}
		if v, err := macVolume("/Volumes/X/a", macStat{"/Volumes/X", fs, macLocal}, f.read); err != nil || v.ID != "uuid:"+v4 {
			t.Errorf("%s, версия 4 — норма: %+v, %v", fs, v, err)
		}
	}
	for _, u := range []string{"", "x", "12345678-1234-"} { // короткая строка не паникует
		if !macUUIDStable("apfs", u) || macUUIDStable("msdos", u) {
			t.Errorf("короткий UUID %q", u)
		}
	}
}

// У96: два тома с одной меткой — разные ID, и поиск по ID первого не отдаёт точку второго.
func TestMacSameLabel(t *testing.T) {
	const u1, u2 = "11111111-1111-1111-1111-111111111111", "22222222-2222-2222-2222-222222222222"
	f := fakeMac{
		uuid: map[string]string{"/Volumes/CARD": u1, "/Volumes/CARD 1": u2},
		name: map[string]string{"/Volumes/CARD": "CARD", "/Volumes/CARD 1": "CARD"},
	}
	s1, s2 := macStat{"/Volumes/CARD", "exfat", macLocal}, macStat{"/Volumes/CARD 1", "exfat", macLocal}
	v1, err1 := macVolume("/Volumes/CARD/a", s1, f.read)
	v2, err2 := macVolume("/Volumes/CARD 1/a", s2, f.read)
	if err1 != nil || err2 != nil || v1.ID == v2.ID || v1.Label != v2.Label {
		t.Fatalf("тома: %+v %v / %+v %v", v1, err1, v2, err2)
	}
	ms := []macStat{{"/", "apfs", macLocal | mntRootFS}, s1, s2}
	p, label, err := macPathFromMounts(ms, v1.ID, v1.Rel, f.read)
	if err != nil || p != "/Volumes/CARD/a" || label != "CARD" {
		t.Errorf("первый: %q %q %v", p, label, err)
	}
	if p, _, err = macPathFromMounts(ms, v2.ID, v2.Rel, f.read); err != nil || p != "/Volumes/CARD 1/a" {
		t.Errorf("второй: %q %v", p, err)
	}
	// Первый вынули — его ID не находит второй.
	if _, _, err = macPathFromMounts([]macStat{s2}, v1.ID, "a", f.read); !errors.Is(err, ErrVolGone) {
		t.Errorf("вынутый том: %v", err)
	}
}

func TestMacPathFromMounts(t *testing.T) {
	id := "uuid:" + testUUIDStr
	f := fakeMac{
		uuid: map[string]string{"/Volumes/A": testUUIDStr, "/Volumes/B": testUUIDStr, "/Volumes/C": "33333333-3333-3333-3333-333333333333", "/": testUUIDStr},
		name: map[string]string{"/Volumes/A": "Арх"},
	}
	a := macStat{"/Volumes/A", "exfat", macLocal}
	b := macStat{"/Volumes/B", "exfat", macLocal}
	boot := macStat{"/", "apfs", macLocal | mntRootFS}
	// Загрузочный том с тем же UUID (подделка) не годится как ответ.
	if p, l, err := macPathFromMounts([]macStat{boot, a}, id, "x/y", f.read); err != nil || p != "/Volumes/A/x/y" || l != "Арх" {
		t.Errorf("обычный: %q %q %v", p, l, err)
	}
	if p, _, err := macPathFromMounts([]macStat{a}, id, "", f.read); err != nil || p != "/Volumes/A" {
		t.Errorf("корень: %q %v", p, err)
	}
	// Клон диска: два тома с одним UUID — путь не угадываем.
	if _, _, err := macPathFromMounts([]macStat{a, b}, id, "x", f.read); !errors.Is(err, ErrVolGone) {
		t.Errorf("клон: %v", err)
	}
	// Пустой список, нет такого, чтение упало.
	if _, _, err := macPathFromMounts(nil, id, "x", f.read); !errors.Is(err, ErrVolGone) {
		t.Errorf("пусто: %v", err)
	}
	if _, _, err := macPathFromMounts([]macStat{{"/Volumes/C", "exfat", macLocal}}, id, "x", f.read); !errors.Is(err, ErrVolGone) {
		t.Errorf("другой UUID: %v", err)
	}
	f.fail = map[string]bool{"/Volumes/A": true}
	if _, _, err := macPathFromMounts([]macStat{a}, id, "x", f.read); !errors.Is(err, ErrVolGone) {
		t.Errorf("ошибка чтения: %v", err)
	}
}

// Список томов за пределом читается не дальше maxMacMounts: чужая таблица не растягивает проход.
func TestMacPathFromMountsLimit(t *testing.T) {
	n := 0
	read := func(string) (volAttr, error) { n++; return volAttr{UUID: "x"}, nil }
	ms := make([]macStat, maxMacMounts*3)
	for i := range ms {
		ms[i] = macStat{"/Volumes/v", "exfat", macLocal}
	}
	_, _, _ = macPathFromMounts(ms, "uuid:y", "", read)
	if n != maxMacMounts {
		t.Errorf("прочитано %d томов, предел %d", n, maxMacMounts)
	}
}

func TestMacStatUp(t *testing.T) {
	st := macStat{"/Volumes/A", "exfat", macLocal}
	var asked []string
	stat := func(p string) (macStat, error) {
		asked = append(asked, p)
		if p == "/Volumes/A" {
			return st, nil
		}
		return macStat{}, &fs.PathError{Op: "statfs", Path: p, Err: fs.ErrNotExist}
	}
	got, err := macStatUp("/Volumes/A/нет/совсем", stat)
	if err != nil || got != st || strings.Join(asked, " ") != "/Volumes/A/нет/совсем /Volumes/A/нет /Volumes/A" {
		t.Errorf("подъём: %+v %v %v", got, err, asked)
	}
	// Иная ошибка (нет доступа) не поднимает путь выше.
	denied := func(p string) (macStat, error) { return macStat{}, fs.ErrPermission }
	if _, err = macStatUp("/Volumes/A/x", denied); !errors.Is(err, fs.ErrPermission) {
		t.Errorf("доступ: %v", err)
	}
	// Корень без тома и бесконечно глубокий путь заканчиваются ошибкой.
	none := func(p string) (macStat, error) { return macStat{}, fs.ErrNotExist }
	if _, err = macStatUp("/a", none); err == nil {
		t.Error("корень без тома: ждали ошибку")
	}
	if _, err = macStatUp("/"+strings.Repeat("a/", maxMacUp*2), none); err == nil {
		t.Error("глубокий путь: ждали ошибку")
	}
}
