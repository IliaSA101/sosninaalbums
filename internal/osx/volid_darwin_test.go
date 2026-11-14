//go:build darwin && cgo

package osx

// Номер тома на настоящих образах macOS (160 Т4, шаг go-mac): hdiutil создаёт и монтирует FAT32 и
// ExFAT. Вне CI без hdiutil — пропуск, в CI — провал: молчаливый пропуск зелёным не считаем.

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// requireHdiutil — путь к hdiutil; нет его: пропуск (CI — провал). Тест не для -short: образ занимает секунды.
func requireHdiutil(t *testing.T) string {
	t.Helper()
	if testing.Short() {
		t.Skip("образы дисков — не для -short")
	}
	p, err := exec.LookPath("hdiutil")
	if err != nil {
		if os.Getenv("CI") != "" {
			t.Fatal("hdiutil не найден в CI: тест тома macOS не должен молчать")
		}
		t.Skip("нет hdiutil")
	}
	return p
}

// hdiutil — вызов с пределом; «Resource busy» (известный сбой раннеров GitHub) повторяем до 3 раз.
func hdiutil(t *testing.T, bin string, args ...string) string {
	t.Helper()
	const tries = 3
	for i := 1; ; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), testkit.Limit(90*time.Second))
		out, err := exec.CommandContext(ctx, bin, args...).CombinedOutput()
		cancel()
		if err == nil {
			return string(out)
		}
		if i >= tries || !strings.Contains(string(out), "Resource busy") {
			t.Fatalf("hdiutil %s: %v\n%s", strings.Join(args, " "), err, out)
		}
		t.Logf("hdiutil %s: Resource busy, повтор %d из %d", args[0], i, tries-1)
		time.Sleep(time.Duration(i) * 2 * time.Second)
	}
}

// macImage — смонтированный образ: точка монтирования и устройство для detach.
type macImage struct {
	t        *testing.T
	bin, dmg string
	dev      string
	Point    string
}

// newMacImage создаёт образ с ФС fsName («MS-DOS FAT32», «ExFAT») и меткой label и монтирует его.
func newMacImage(t *testing.T, bin, fsName, label string) *macImage {
	t.Helper()
	dir := testkit.TempDir(t)
	if dir == "" {
		t.Fatal("нет временной папки")
	}
	m := &macImage{t: t, bin: bin, dmg: filepath.Join(dir, "img.dmg")}
	hdiutil(t, bin, "create", "-size", "64m", "-fs", fsName, "-volname", label, "-type", "UDIF", m.dmg)
	t.Cleanup(m.detach)
	m.attach()
	return m
}

// attach монтирует образ без значка в Finder; точка — последняя колонка строки с /Volumes/.
func (m *macImage) attach() {
	m.t.Helper()
	out := hdiutil(m.t, m.bin, "attach", "-nobrowse", "-noverify", m.dmg)
	for _, line := range strings.Split(out, "\n") {
		cols := strings.Split(strings.TrimRight(line, " \r"), "\t")
		last := strings.TrimSpace(cols[len(cols)-1])
		if strings.HasPrefix(last, "/Volumes/") {
			m.dev = strings.Fields(cols[0])[0]
			m.Point = last
		}
	}
	if m.Point == "" {
		m.t.Fatalf("hdiutil attach не назвал точку монтирования:\n%s", out)
	}
	if real, err := filepath.EvalSymlinks(m.Point); err == nil {
		m.Point = real
	}
}

func (m *macImage) detach() {
	if m.dev == "" {
		return
	}
	// Cleanup не должен падать или виснуть: каждый вызов под пределом, повтор с -force снимает занятость Spotlight.
	run := func(args ...string) error {
		ctx, cancel := context.WithTimeout(context.Background(), testkit.Limit(60*time.Second))
		defer cancel()
		return exec.CommandContext(ctx, m.bin, args...).Run()
	}
	if err := run("detach", m.dev); err != nil {
		_ = run("detach", "-force", m.dev)
	}
	m.dev, m.Point = "", ""
}

// dumpMacMounts — отладка провала: все тома, их флаги, UUID по getattrlist и diskutil info точки.
func dumpMacMounts(t *testing.T, point string) {
	t.Helper()
	ms, err := listMacMounts()
	if err != nil {
		t.Logf("getfsstat: %v", err)
	}
	for _, st := range ms {
		a, aerr := readVolAttr(st.Point)
		t.Logf("mount %q fs=%s flags=%#x bindable=%v uuid=%q name=%q err=%v", st.Point, st.Fstype, st.Flags, macBindable(st), a.UUID, a.Name, aerr)
	}
	if point != "" {
		out, _ := exec.Command("diskutil", "info", point).CombinedOutput()
		t.Logf("diskutil info %s:\n%s", point, out)
	}
}

// waitVolumePath опрашивает VolumePath до успеха или предела; при провале печатает состояние всех томов.
func waitVolumePath(t *testing.T, id, rel, point string) (p, label string, err error) {
	t.Helper()
	deadline := time.Now().Add(testkit.Limit(20 * time.Second))
	for n := 1; ; n++ {
		p, label, err = VolumePath(id, rel)
		if err == nil {
			if n > 1 {
				t.Logf("VolumePath нашёл том с %d-го опроса", n)
			}
			return p, label, nil
		}
		if time.Now().After(deadline) {
			t.Logf("номер до detach: %s, опросов %d", id, n)
			dumpMacMounts(t, point)
			return p, label, err
		}
		time.Sleep(200 * time.Millisecond)
	}
}

func TestVolumeDarwinImage(t *testing.T) {
	bin := requireHdiutil(t)
	for _, fsName := range []string{"MS-DOS FAT32", "ExFAT"} {
		t.Run(fsName, func(t *testing.T) {
			const label = "SATEST"
			m := newMacImage(t, bin, fsName, label)
			sub := filepath.Join(m.Point, "Shoot", "2026")
			if err := os.MkdirAll(sub, 0o755); err != nil {
				t.Fatal(err)
			}
			// UUID FAT/exFAT на macOS 15 иногда случайный (версия 4, 160-mac-N+1): такой том честно «не опознан».
			stable, uuid := imageUUIDStable(t, m.Point)
			if !stable {
				t.Logf("случайный UUID %s у %s: проверяем честный отказ", uuid, fsName)
				if v, err := VolumeOf(sub); !errors.Is(err, errNoVolID) {
					t.Fatalf("случайный UUID: VolumeOf = %+v, %v (ждали errNoVolID)", v, err)
				}
				return
			}
			v, err := VolumeOf(sub)
			if err != nil {
				t.Fatalf("VolumeOf: %v", err)
			}
			if !strings.HasPrefix(v.ID, "uuid:") || len(v.ID) != len("uuid:")+36 || v.Label != label || v.Rel != "Shoot/2026" || v.Root != m.Point {
				t.Fatalf("том: %+v", v)
			}
			// Несуществующая папка на томе: важен только том.
			if v2, err := VolumeOf(filepath.Join(m.Point, "Нет", "такой")); err != nil || v2.ID != v.ID || v2.Rel != "Нет/такой" {
				t.Errorf("несуществующая папка: %+v, %v", v2, err)
			}
			p, l, err := VolumePath(v.ID, v.Rel)
			if err != nil || p != sub || l != label {
				t.Fatalf("VolumePath: %q %q %v", p, l, err)
			}
			// Вынули и вставили снова: тот же номер, путь находится (точка может смениться).
			m.detach()
			if _, _, err := VolumePath(v.ID, v.Rel); !errors.Is(err, ErrVolGone) {
				t.Errorf("после detach: %v", err)
			}
			m.attach()
			// После attach драйвер мог выдать случайный UUID: тогда том не находится по старому номеру и не опознаётся, это не ошибка.
			if stable2, uuid2 := imageUUIDStable(t, m.Point); !stable2 {
				t.Logf("после attach UUID случайный (%s, был %s): ждём честный отказ", uuid2, v.ID)
				if _, _, err := VolumePath(v.ID, v.Rel); !errors.Is(err, ErrVolGone) {
					t.Errorf("случайный UUID после attach: VolumePath = %v, ждали ErrVolGone", err)
				}
				if v3, err := VolumeOf(filepath.Join(m.Point, "Shoot")); !errors.Is(err, errNoVolID) {
					t.Errorf("случайный UUID после attach: VolumeOf = %+v, %v", v3, err)
				}
				return
			}
			// Том виден в getfsstat не мгновенно после attach (DiskArbitration может перемонтировать после fsck): ждём условие, не срок.
			p, l, err = waitVolumePath(t, v.ID, v.Rel, m.Point)
			if err != nil || !strings.HasPrefix(p, m.Point) || !strings.HasSuffix(p, "Shoot/2026") || l != label {
				t.Fatalf("после повторного attach: %q %q %v (точка %s)", p, l, err, m.Point)
			}
			if v3, err := VolumeOf(p); err != nil || v3.ID != v.ID {
				t.Errorf("тот же том после attach: %+v, %v (ждали %s)", v3, err, v.ID)
			}
		})
	}
}

// imageUUIDStable — читает UUID и ФС тома по точке тем же путём, что продукт, и говорит, годен ли он для опознания.
func imageUUIDStable(t *testing.T, point string) (stable bool, uuid string) {
	t.Helper()
	st, err := macStatUp(point, statMac)
	if err != nil {
		t.Fatalf("statfs %s: %v", point, err)
	}
	a, err := readVolAttr(st.Point)
	if err != nil {
		t.Fatalf("getattrlist %s: %v", st.Point, err)
	}
	return macUUIDStable(st.Fstype, a.UUID), a.UUID
}

// У96: два образа с одной меткой (второй встанет в «/Volumes/<метка> 1») — разные номера, и номер
// первого не находит путь второго.
func TestVolumeDarwinSameLabel(t *testing.T) {
	bin := requireHdiutil(t)
	const label = "SAPAIR"
	a := newMacImage(t, bin, "MS-DOS FAT32", label)
	b := newMacImage(t, bin, "MS-DOS FAT32", label)
	if a.Point == b.Point {
		t.Fatalf("оба образа в %s", a.Point)
	}
	// Случайный UUID FAT (160-mac-N+1) — том не опознан, сравнивать нечего.
	for _, m := range []*macImage{a, b} {
		if stable, uuid := imageUUIDStable(t, m.Point); !stable {
			t.Logf("случайный UUID %s у %s: проверяем честный отказ", uuid, m.Point)
			if v, err := VolumeOf(filepath.Join(m.Point, "x")); !errors.Is(err, errNoVolID) {
				t.Fatalf("случайный UUID: VolumeOf = %+v, %v (ждали errNoVolID)", v, err)
			}
			return
		}
	}
	va, err := VolumeOf(filepath.Join(a.Point, "x"))
	if err != nil {
		t.Fatal(err)
	}
	vb, err := VolumeOf(filepath.Join(b.Point, "x"))
	if err != nil {
		t.Fatal(err)
	}
	if va.ID == vb.ID || va.Label != vb.Label {
		t.Fatalf("метки равны, номера должны различаться: %+v / %+v", va, vb)
	}
	if p, _, err := VolumePath(va.ID, "x"); err != nil || p != filepath.Join(a.Point, "x") {
		t.Errorf("первый: %q %v (ждали %s)", p, err, a.Point)
	}
	if p, _, err := VolumePath(vb.ID, "x"); err != nil || p != filepath.Join(b.Point, "x") {
		t.Errorf("второй: %q %v (ждали %s)", p, err, b.Point)
	}
	// Первый вынули: его номер не отдаёт точку второго с той же меткой.
	a.detach()
	if p, _, err := VolumePath(va.ID, "x"); !errors.Is(err, ErrVolGone) {
		t.Errorf("после detach первого: %q %v", p, err)
	}
}

// Загрузочный том и домашняя папка — без номера (привязка там ничего не даёт, 0.36-160-7).
func TestVolumeDarwinBootNoID(t *testing.T) {
	for _, p := range []string{"/", "/Users", testkit.TempDir(t)} {
		if v, err := VolumeOf(p); err == nil {
			t.Errorf("VolumeOf(%s) = %+v, ждали отказ", p, v)
		}
	}
}
