package osx

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// fakeVolDirs — поддельные /dev, by-uuid и by-label: файлы-«устройства» и симлинки на них, как у udev.
func fakeVolDirs(t *testing.T) volDirs {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("симлинки устройств и пути «/dev» — Linux")
	}
	root := testkit.TempDir(t)
	d := volDirs{dev: filepath.Join(root, "dev"), uuid: filepath.Join(root, "dev", "disk", "by-uuid"), label: filepath.Join(root, "dev", "disk", "by-label")}
	for _, dir := range []string{d.uuid, d.label} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, dev := range []string{"sda2", "sdb1", "sdc1", "sdd1", "sde1"} {
		if err := os.WriteFile(filepath.Join(d.dev, dev), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	// by-uuid ведёт на sda2, sdb1, sdc1, sde1; у sdd1 (FAT, без UUID) записи нет. sdb1 — с меткой.
	link := func(dir, name, dev string) {
		if err := os.Symlink(filepath.Join("..", "..", dev), filepath.Join(dir, name)); err != nil {
			t.Fatal(err)
		}
	}
	link(d.uuid, "11111111-2222-3333-4444-555555555555", "sda2")
	link(d.uuid, "AAAA-0001", "sdb1")
	link(d.uuid, "01D9A8B7C6D5E4F3", "sdc1")
	link(d.uuid, "5e5e5e5e-0000-4000-8000-000000000005", "sde1")
	link(d.label, `Фото\x20архив`, "sdb1")
	// Псевдоним «/dev/mapper/vg-root» на sde1: источник в mountinfo идёт через симлинк.
	if err := os.MkdirAll(filepath.Join(d.dev, "mapper"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join("..", "sde1"), filepath.Join(d.dev, "mapper", "vg-root")); err != nil {
		t.Fatal(err)
	}
	return d
}

// volMountinfo — образцы строк: DEV заменяется на каталог поддельных устройств.
const volMountinfo = `22 1 8:2 / / rw,relatime shared:1 - ext4 DEV/sda2 rw
24 22 8:17 / /media/u/DISK rw,nosuid,nodev shared:3 - exfat DEV/sdb1 rw
25 22 8:33 / /media/u/WIN rw,nosuid shared:4 - fuseblk DEV/sdc1 rw,user_id=0
26 22 8:17 /Съёмки/2026 /mnt/bind rw,relatime shared:5 - exfat DEV/sdb1 rw
27 22 8:49 / /media/u/My\040Disk rw,nosuid shared:6 - vfat DEV/sdd1 rw
28 22 0:53 / /tmp rw,nosuid,nodev - tmpfs tmpfs rw
29 22 0:60 / /tank rw - zfs tank/data rw
30 22 0:61 / /mnt/nas rw - nfs4 srv:/export rw
31 22 0:62 / /mnt/smb rw - cifs //srv/share rw
32 22 8:65 /@home /home rw - btrfs DEV/mapper/vg-root rw,subvol=/@home
`

func volMounts(t *testing.T, d volDirs) []mount {
	t.Helper()
	ms, err := parseMountinfo(strings.NewReader(strings.ReplaceAll(volMountinfo, "DEV", d.dev)))
	if err != nil {
		t.Fatal(err)
	}
	return ms
}

// TestVolIDMountinfo — том по таблице: ext4, bind, fuseblk (ntfs-3g), tmpfs, zfs, сеть, подтом
// btrfs через симлинк устройства; метка из by-label; нет UUID — ошибка.
func TestVolIDMountinfo(t *testing.T) {
	d := fakeVolDirs(t)
	ms := volMounts(t, d)
	const rootID = "uuid:11111111-2222-3333-4444-555555555555"

	ok := []struct {
		name, path string
		want       Vol
	}{
		{"ext4 корень", "/srv/Фото/2026", Vol{Root: "/", ID: rootID, Rel: "srv/Фото/2026"}},
		{"сам корень тома", "/media/u/DISK", Vol{Root: "/media/u/DISK", ID: "uuid:AAAA-0001", Label: "Фото архив"}},
		{"внутри exfat", "/media/u/DISK/Съёмки/2026/Петровы", Vol{Root: "/media/u/DISK", ID: "uuid:AAAA-0001", Label: "Фото архив", Rel: "Съёмки/2026/Петровы"}},
		{"bind: корень поддерева в Rel", "/mnt/bind/Петровы", Vol{Root: "/mnt/bind", ID: "uuid:AAAA-0001", Label: "Фото архив", Rel: "Съёмки/2026/Петровы"}},
		{"fuseblk (ntfs-3g)", "/media/u/WIN/Photos", Vol{Root: "/media/u/WIN", ID: "uuid:01D9A8B7C6D5E4F3", Rel: "Photos"}},
		{"btrfs подтом через симлинк", "/home/u/x", Vol{Root: "/home", ID: "uuid:5e5e5e5e-0000-4000-8000-000000000005", Rel: "@home/u/x"}},
		{"папки нет — том известен", "/media/u/DISK/нет/такой", Vol{Root: "/media/u/DISK", ID: "uuid:AAAA-0001", Label: "Фото архив", Rel: "нет/такой"}},
	}
	for _, c := range ok {
		got, err := volFromMounts(ms, c.path, d)
		if err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		if got != c.want {
			t.Errorf("%s: %+v, want %+v", c.name, got, c.want)
		}
	}

	// Без UUID или не блочное устройство: ошибка, а не пустой ID.
	for _, p := range []string{"/media/u/My Disk/x", "/tmp/x", "/tank/x", "/mnt/nas/x", "/mnt/smb/x"} {
		if v, err := volFromMounts(ms, p, d); !errors.Is(err, errNoVolID) {
			t.Errorf("%s: %+v, %v; want errNoVolID", p, v, err)
		}
	}
	// Нет каталога by-uuid вовсе (контейнер) — тоже ошибка.
	d2 := d
	d2.uuid = filepath.Join(testkit.TempDir(t), "нет")
	if _, err := volFromMounts(ms, "/media/u/DISK/x", d2); !errors.Is(err, errNoVolID) {
		t.Errorf("без by-uuid: %v", err)
	}
	// Пустая таблица — ошибка.
	if _, err := volFromMounts(nil, "/x", d); err == nil {
		t.Error("пустая таблица принята")
	}
}

// TestVolPathMountinfo — обратный поиск: где сейчас том id; bind и подтом учитывают корень поддерева.
func TestVolPathMountinfo(t *testing.T) {
	d := fakeVolDirs(t)
	ms := volMounts(t, d)

	cases := []struct {
		id, rel, want, label string
	}{
		{"uuid:AAAA-0001", "Съёмки/2026/Петровы", "/mnt/bind/Петровы", "Фото архив"}, // самый глубокий корень точнее
		{"uuid:AAAA-0001", "Другое/x", "/media/u/DISK/Другое/x", "Фото архив"},
		{"uuid:AAAA-0001", "", "/media/u/DISK", "Фото архив"},
		{"uuid:01D9A8B7C6D5E4F3", "Photos", "/media/u/WIN/Photos", ""},
		{"uuid:5e5e5e5e-0000-4000-8000-000000000005", "@home/u", "/home/u", ""},
		{"uuid:11111111-2222-3333-4444-555555555555", "home/u", "/home/u", ""},
	}
	for _, c := range cases {
		got, label, err := pathFromMounts(ms, c.id, c.rel, d)
		if err != nil || got != c.want || label != c.label {
			t.Errorf("%s %q: %q, %q, %v; want %q, %q", c.id, c.rel, got, label, err, c.want, c.label)
		}
	}
	// Тома нет (флешку вынули), UUID чужой, подтом btrfs не охватывает rel вне @home.
	for _, c := range []struct{ id, rel string }{
		{"uuid:ZZZZ-9999", "x"},
		{"win:00000001:ntfs", "x"},
		{"uuid:5e5e5e5e-0000-4000-8000-000000000005", "другой/подтом"},
	} {
		if p, _, err := pathFromMounts(ms, c.id, c.rel, d); !errors.Is(err, ErrVolGone) {
			t.Errorf("%s %q: %q, %v; want ErrVolGone", c.id, c.rel, p, err)
		}
	}
}

// TestVolMountinfoBad — битые, усечённые и пустые входы: ошибка или «нет тома», без паники.
func TestVolMountinfoBad(t *testing.T) {
	d := fakeVolDirs(t)
	inputs := []string{
		"",
		"\n\n",
		"22 1 8:2 / / rw - ext4", // нет источника
		"22 1 8:2 / / rw -",      // усечено после «-»
		"22 1",                   // усечено до точки
		"- - - - - - -",
		"22 1 8:2 / / rw - ext4 " + strings.Repeat("я", 5000), // огромный источник
		"22 1 8:2 \\ /\\x rw - ext4 \\134dev\\134sda2 rw",
	}
	for _, in := range inputs {
		ms, err := parseMountinfo(strings.NewReader(in))
		if err != nil {
			continue
		}
		for _, p := range []string{"/", "/x", "/media/u/DISK"} {
			if _, err := volFromMounts(ms, p, d); err == nil {
				t.Errorf("%q: %s принят", in, p)
			}
		}
		if _, _, err := pathFromMounts(ms, "uuid:AAAA-0001", "x", d); !errors.Is(err, ErrVolGone) {
			t.Errorf("%q: pathFromMounts: %v", in, err)
		}
	}
}

// TestVolumeOfInput — проверки до вызова ОС: пустой, относительный, с NUL, длиннее предела пути;
// ID и Rel вне пределов. Любая ОС: ошибка раньше обращения к системе.
func TestVolumeOfInput(t *testing.T) {
	for _, p := range []string{"", "относительный/путь", "x", "/a\x00b", "/" + strings.Repeat("a", maxVolPath)} {
		if v, err := VolumeOf(p); err == nil {
			t.Errorf("VolumeOf(%.20q) = %+v, want error", p, v)
		}
	}
	for _, c := range []struct{ id, rel string }{
		{"", ""},
		{"uuid:" + strings.Repeat("a", maxVolID), ""},
		{"uuid:\x01", ""},
		{"uuid:\xff", ""},
		{"uuid:AAAA-0001", "/абсолютный"},
		{"uuid:AAAA-0001", "a/../b"},
		{"uuid:AAAA-0001", ".."},
		{"uuid:AAAA-0001", "a\x00"},
		{"uuid:AAAA-0001", strings.Repeat("a", maxVolRel+1)},
	} {
		if p, _, err := VolumePath(c.id, c.rel); err == nil {
			t.Errorf("VolumePath(%.20q, %.20q) = %q, want error", c.id, c.rel, p)
		}
	}
}

// TestVolidHelpers — метка: управляющие знаки и порча UTF-8 убираются, длина в пределе; udev-экранирование.
func TestVolidHelpers(t *testing.T) {
	if got := cleanLabel("Фото\x00\n\x7f архив\xff"); got != "Фото архив" {
		t.Errorf("cleanLabel = %q", got)
	}
	if got := cleanLabel(strings.Repeat("я", 1000)); len(got) > maxVolID || !validVolID(got) {
		t.Errorf("метка длиннее предела или негодна: %d", len(got))
	}
	for in, want := range map[string]string{
		`a\x20b`:   "a b",
		`a\x2`:     `a\x2`,
		`a\xZZb`:   `a\xZZb`,
		`\x41\x42`: "AB",
		"":         "",
	} {
		if got := unescapeLabel(in); got != want {
			t.Errorf("unescapeLabel(%q) = %q, want %q", in, got, want)
		}
	}
	for _, s := range []string{"", "a b", "a/b", strings.Repeat("a", 65)} {
		if validUUID(s) {
			t.Errorf("validUUID(%q) = true", s)
		}
	}
}

func FuzzVolMountinfo(f *testing.F) {
	f.Add(strings.ReplaceAll(volMountinfo, "DEV", "/dev"))
	f.Add("22 1 8:2 / / rw - ext4 /dev/sda2 rw")
	f.Add("")
	d := volDirs{dev: "/dev", uuid: "/nonexistent/by-uuid", label: "/nonexistent/by-label"}
	f.Fuzz(func(t *testing.T, in string) {
		ms, err := parseMountinfo(strings.NewReader(in))
		if err != nil {
			return
		}
		_, _ = volFromMounts(ms, "/media/u/DISK/x", d)
		_, _, _ = pathFromMounts(ms, "uuid:AAAA-0001", "x/y", d)
	})
}
