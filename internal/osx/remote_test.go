package osx

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Образец /proc/self/mountinfo: корень ext4, отдельный /home на btrfs, диск exFAT в /media/u/DISK,
// ntfs-3g (fuseblk), сеть: nfs4, cifs, sshfs и gvfs (fuse.*), точка с пробелом, tmpfs.
const sampleMountinfo = `22 1 8:2 / / rw,relatime shared:1 - ext4 /dev/sda2 rw
23 22 8:3 / /home rw,relatime shared:2 - btrfs /dev/sda3 rw,subvolid=5
24 22 8:17 / /media/u/DISK rw,nosuid,nodev shared:3 - exfat /dev/sdb1 rw
25 22 8:33 / /media/u/WIN rw,nosuid shared:4 - fuseblk /dev/sdc1 rw,user_id=0
26 22 0:50 / /mnt/nas rw,relatime shared:5 - nfs4 srv:/export rw,vers=4.2
27 22 0:51 / /mnt/smb rw,relatime shared:6 - cifs //srv/share rw
28 22 0:52 / /home/u/ssh rw,nosuid,nodev - fuse.sshfs u@srv:/ rw,user_id=1000
29 22 0:48 / /run/user/1000/gvfs rw,nosuid,nodev - fuse.gvfsd-fuse gvfsd-fuse rw
30 22 8:49 / /media/u/My\040Disk rw,nosuid shared:7 - vfat /dev/sdd1 rw
31 22 0:53 / /tmp rw,nosuid,nodev - tmpfs tmpfs rw
32 23 0:54 / /home/u/Docs rw - ext4 /dev/sde1 rw
bad line without separator
33 22 0:55 / /broken rw
`

func TestRemoteMountinfo(t *testing.T) {
	ms, err := parseMountinfo(strings.NewReader(sampleMountinfo))
	if err != nil {
		t.Fatal(err)
	}
	if len(ms) != 11 {
		t.Fatalf("строк разобрано %d, ждали 11 (битые пропущены): %v", len(ms), ms)
	}
	for _, c := range []struct {
		path   string
		remote bool
	}{
		{"/", false}, {"/etc/x", false}, // корень ext4
		{"/home/u/Фото", false},                 // btrfs
		{"/home/u/Docs/2026", false},            // ext4 поверх btrfs: длиннейший префикс
		{"/media/u/DISK/Свадьба", false},        // exFAT
		{"/media/u/WIN/Фото", false},            // fuseblk — ntfs-3g
		{"/media/u/My Disk/Фото", false},        // \040 в точке монтирования
		{"/media/u/DISKETA", false},             // не внутри /media/u/DISK, а на корне ext4
		{"/mnt/nas/Фото", true},                 // nfs4
		{"/mnt/smb", true},                      // cifs, сама точка
		{"/home/u/ssh/Фото", true},              // fuse.sshfs
		{"/run/user/1000/gvfs/smb-share", true}, // fuse.gvfsd-fuse
		{"/tmp/x", true},                        // tmpfs не в списке разрешённых
	} {
		if got := remoteByMounts(ms, c.path); got != c.remote {
			t.Errorf("%s: remote = %v, ждали %v", c.path, got, c.remote)
		}
	}
	// Тип без точки в таблице (пустая таблица) — сетевой.
	if !remoteByMounts(nil, "/x") {
		t.Error("пустая таблица: ждали «сетевой»")
	}
}

// Края разбора: пусто, усечённые строки, огромная строка, огромный файл — ошибка или пропуск, не паника.
func TestMountinfoLimits(t *testing.T) {
	for name, in := range map[string]string{
		"пусто":     "",
		"без конца": "22 1 8:2 / / rw -",
		"усечена":   "22 1 8:2 /",
		"только -":  "-\n-\n",
	} {
		ms, err := parseMountinfo(strings.NewReader(in))
		if err != nil || len(ms) != 0 {
			t.Errorf("%s: %v, %v; ждали пусто без ошибки", name, ms, err)
		}
	}
	long := "22 1 8:2 / /" + strings.Repeat("a", maxMountinfoLine) + " rw - ext4 x rw\n"
	if _, err := parseMountinfo(strings.NewReader(long)); err == nil {
		t.Error("строка длиннее предела: ждали ошибку")
	}
	line := "22 1 8:2 / /p rw - ext4 x rw\n"
	big := strings.Repeat(line, maxMountinfo/len(line)+10)
	if _, err := parseMountinfo(strings.NewReader(big)); err == nil {
		t.Error("файл больше предела: ждали ошибку")
	}
	if got := unescapeMount(`/a\040b\134c\0`); got != `/a b\c\0` {
		t.Errorf("unescapeMount = %q", got)
	}
}

// Файл таблицы не читается (нет, каталог) — «сетевой», не «локальный» и не паника.
func TestRemoteErrorIsRemote(t *testing.T) {
	old := mountinfoFile
	t.Cleanup(func() { mountinfoFile = old })
	for name, file := range map[string]string{
		"нет файла": filepath.Join(testkit.TempDir(t), "нет"),
		"каталог":   testkit.TempDir(t),
	} {
		mountinfoFile = file
		if _, err := MountPoints(); err == nil && name == "нет файла" {
			t.Errorf("%s: MountPoints без ошибки", name)
		}
	}
	// Проба с ошибкой проверки и с паникой отвечает «сетевой».
	for name, check := range map[string]func(string) (bool, error){
		"ошибка": func(string) (bool, error) { return false, os.ErrPermission },
		"паника": func(string) (bool, error) { panic("чужой том") },
	} {
		p := &remoteProbe{check: check, timeout: time.Second}
		if !p.remote("/x") {
			t.Errorf("%s: ждали «сетевой»", name)
		}
		if p.busy.Load() {
			t.Errorf("%s: проба не освободила слот", name)
		}
	}
	// Пустой и относительный путь — тоже.
	if !Remote("") || !Remote("rel/dir") {
		t.Error("пустой и относительный путь: ждали «сетевой»")
	}
}

// Одна проба за раз: пока первая висит, вторая сразу «сетевой» и проверка не запускается ещё раз.
func TestRemoteOneProbe(t *testing.T) {
	var calls atomic.Int32
	release := make(chan struct{})
	var once sync.Once
	t.Cleanup(func() { once.Do(func() { close(release) }) })
	p := &remoteProbe{timeout: 50 * time.Millisecond, check: func(string) (bool, error) {
		calls.Add(1)
		<-release
		return false, nil
	}}
	if !p.remote("/a") { // первая не уложилась в предел
		t.Fatal("зависшая проба: ждали «сетевой»")
	}
	start := time.Now()
	if !p.remote("/b") {
		t.Fatal("вторая при висящей первой: ждали «сетевой»")
	}
	if d := time.Since(start); d > testkit.Limit(40*time.Millisecond) {
		t.Errorf("вторая ждала %v, ждали мгновенный отказ", d)
	}
	if n := calls.Load(); n != 1 {
		t.Fatalf("проверок запущено %d, ждали 1", n)
	}
	once.Do(func() { close(release) })
	waitFree := time.After(testkit.Limit(5 * time.Second))
	for p.busy.Load() {
		select {
		case <-waitFree:
			t.Fatal("слот не освободился после завершения пробы")
		case <-time.After(time.Millisecond):
		}
	}
	p.timeout = time.Second
	if p.remote("/c") { // проба отпущена и вернула «локальный»
		t.Error("после освобождения: ждали «локальный»")
	}
	if n := calls.Load(); n != 2 {
		t.Errorf("проверок %d, ждали 2", n)
	}
}

// Параллельные вызовы: проверка идёт одна, прочие — «сетевой», гонок нет (-race).
func TestRemoteParallel(t *testing.T) {
	var running, peak atomic.Int32
	p := &remoteProbe{timeout: time.Second, check: func(string) (bool, error) {
		n := running.Add(1)
		for {
			old := peak.Load()
			if n <= old || peak.CompareAndSwap(old, n) {
				break
			}
		}
		time.Sleep(5 * time.Millisecond)
		running.Add(-1)
		return false, nil
	}}
	var wg sync.WaitGroup
	for range 16 {
		wg.Go(func() { p.remote("/x") })
	}
	wg.Wait()
	if peak.Load() != 1 {
		t.Errorf("одновременных проверок %d, ждали 1", peak.Load())
	}
}

func TestDarwinRemote(t *testing.T) {
	for _, c := range []struct {
		flags  uint32
		fstype string
		remote bool
	}{
		{mntLocal, "apfs", false}, {mntLocal, "hfs", false}, {mntLocal, "exfat", false}, {mntLocal, "msdos", false},
		{0, "apfs", true},         // не локальный по флагу
		{mntLocal, "smbfs", true}, // локальным отмечен, но типа нет в списке
		{mntLocal, "", true},
		{0, "nfs", true},
	} {
		if got := darwinRemote(c.flags, c.fstype); got != c.remote {
			t.Errorf("%#x %q: %v, ждали %v", c.flags, c.fstype, got, c.remote)
		}
	}
}

func FuzzParseMountinfo(f *testing.F) {
	f.Add(sampleMountinfo)
	f.Add("22 1 8:2 / /a\\040b rw - ext4 x rw\n")
	f.Add("- - - - - - -\n")
	f.Fuzz(func(t *testing.T, s string) {
		ms, err := parseMountinfo(strings.NewReader(s))
		if err == nil {
			_ = remoteByMounts(ms, "/a/b")
		}
	})
}

// На этой машине таблица читается, а корень не пуст (Linux) — иначе Remote всегда был бы «сетевой».
func TestMountPointsThisHost(t *testing.T) {
	pts, err := MountPoints()
	if err != nil {
		t.Skipf("таблица недоступна: %v", err)
	}
	if len(pts) == 0 && runtime.GOOS == "linux" {
		t.Error("на Linux ждали хотя бы корень")
	}
}
