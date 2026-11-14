package app

import (
	"errors"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// heicBytes — заголовок HEIC (brand heic) без данных: сигнатура есть, картинки нет.
func heicBytes() []byte {
	return append([]byte("\x00\x00\x00\x18ftypheic\x00\x00\x00\x00"), make([]byte, 16)...)
}

// waitRead ждёт конца чтения целиком: итог unread публикуется после финального отчёта scan.
func waitRead(t *testing.T, a *App) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		a.mu.Lock()
		busy := a.scanning
		a.mu.Unlock()
		if !busy && !a.GetState().Scan.Active {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("чтение не закончилось")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// lastUnread — данные последнего события `unread`; ok = false — событий не было.
func lastUnread(t *testing.T, evs func(string) []evRec) (m map[string]string, ok bool) {
	t.Helper()
	all := evs("unread")
	if len(all) == 0 {
		return nil, false
	}
	m, isMap := all[len(all)-1].data[0].(map[string]string)
	if !isMap || m == nil {
		t.Fatalf("данные события unread: %#v, ждали непустую карту (даже пустую)", all[len(all)-1].data[0])
	}
	return m, true
}

// Отдельный HEIC под .jpg: State.unread и событие unread несут причину под ID из вида; заменили
// файл настоящим JPEG — после чтения запись уходит, пустое событие приходит.
func TestLooseUnreadState(t *testing.T) {
	a, evs := evApp(t)
	dir := testkit.TempDir(t)
	heic := filepath.Join(dir, "heic.jpg")
	empty := filepath.Join(dir, "empty.jpg")
	txt := filepath.Join(dir, "txt.png")
	for p, data := range map[string][]byte{heic: heicBytes(), empty: nil, txt: []byte("не картинка, просто текст")} {
		if err := os.WriteFile(p, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	good := testkit.JPEG(t, dir, "good.jpg", 20, 10)
	if _, err := a.AddFilePaths([]string{heic, empty, txt, good}); err != nil {
		t.Fatal(err)
	}
	waitRead(t, a)

	st := a.GetState()
	if len(st.Unread) != 3 || st.Unread[library.IDFor(good)] != "" {
		t.Fatalf("State.Unread: %v", st.Unread)
	}
	// Ключи — те же ID, что отдаёт вид (LooseFileView.Photo): по ним окно ищет свою строку.
	view := a.editor().View()
	if len(view.Loose) != 1 || len(view.Loose[0].Files) != 4 {
		t.Fatalf("вид: %+v", view.Loose)
	}
	byPath := map[string]string{}
	for _, f := range view.Loose[0].Files {
		byPath[f.Path] = f.Photo
	}
	for path, want := range map[string]string{heic: "внутри HEIC", empty: "пустой файл", txt: "не похоже на картинку"} {
		if got := st.Unread[byPath[path]]; !strings.HasPrefix(got, want) || !strings.Contains(got, "сохраните как JPEG") {
			t.Errorf("%s: причина %q, ждали начало %q", filepath.Base(path), got, want)
		}
	}
	ev, ok := lastUnread(t, evs)
	if !ok || !maps.Equal(ev, st.Unread) {
		t.Fatalf("событие unread: %v (было ли: %v), State: %v", ev, ok, st.Unread)
	}

	// Файл заменён настоящим JPEG (другие размер и время): после чтения записи нет, событие пустое.
	testkit.JPEG(t, dir, "heic.jpg", 40, 30)
	a.Rescan()
	waitRead(t, a)
	st = a.GetState()
	if _, still := st.Unread[library.IDFor(heic)]; still || len(st.Unread) != 2 {
		t.Fatalf("после замены файла: %v", st.Unread)
	}
	if ev, _ := lastUnread(t, evs); !maps.Equal(ev, st.Unread) {
		t.Fatalf("событие после замены: %v, State: %v", ev, st.Unread)
	}

	// Все файлы читаются: событие не пропущено и пусто (иначе в окне остались бы старые записи).
	for _, p := range []string{empty, txt} {
		testkit.JPEG(t, dir, filepath.Base(p), 20, 20)
	}
	a.Rescan()
	waitRead(t, a)
	ev, ok = lastUnread(t, evs)
	if !ok || len(ev) != 0 || len(a.GetState().Unread) != 0 {
		t.Fatalf("после починки всех: событие %v, State %v", ev, a.GetState().Unread)
	}
}

// Пропуски в обычных папках в unread не попадают (они только в журнале); новый альбом чужих причин не несёт.
func TestLooseUnreadOnlyLoose(t *testing.T) {
	a, evs := evApp(t)
	root := testkit.TempDir(t)
	folder := filepath.Join(root, "Папка")
	if err := os.Mkdir(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	inFolder := filepath.Join(folder, "bad.jpg")
	if err := os.WriteFile(inFolder, heicBytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	testkit.JPEG(t, folder, "ok.jpg", 20, 10)
	loose := filepath.Join(root, "loose.jpg")
	if err := os.WriteFile(loose, heicBytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := a.editor().AddFolder(folder); err != nil {
		t.Fatal(err)
	}
	if _, err := a.AddFilePaths([]string{loose}); err != nil {
		t.Fatal(err)
	}
	waitRead(t, a)
	st := a.GetState()
	if len(st.Unread) != 1 || st.Unread[library.IDFor(loose)] == "" {
		t.Fatalf("State.Unread: %v, ждали только отдельный файл", st.Unread)
	}
	if _, has := st.Unread[library.IDFor(inFolder)]; has {
		t.Fatal("пропуск из папки попал в unread")
	}
	if ev, _ := lastUnread(t, evs); !maps.Equal(ev, st.Unread) {
		t.Fatalf("событие: %v, State: %v", ev, st.Unread)
	}

	// Только папка: пропуск в ней есть, unread пуст (и событие пустое).
	a.RemoveLoose(a.editor().View().Loose[0].ID, "")
	a.Rescan()
	waitRead(t, a)
	if len(a.GetState().Unread) != 0 {
		t.Fatalf("после удаления отдельного файла: %v", a.GetState().Unread)
	}
	if ev, ok := lastUnread(t, evs); !ok || len(ev) != 0 {
		t.Fatalf("событие после удаления: %v %v", ev, ok)
	}
}

// Другой альбом: причины прежнего не остаются в State, пока идёт новое чтение.
func TestUnreadPrunedOnSourcesChange(t *testing.T) {
	keep, gone := "/a/keep.jpg", "/a/gone.jpg"
	cur := map[string]string{library.IDFor(keep): "k", library.IDFor(gone): "g"}
	got := prunedUnread(cur, library.Sources{Files: []string{keep}})
	if len(got) != 1 || got[library.IDFor(keep)] != "k" {
		t.Errorf("оставлено %v", got)
	}
	if got := prunedUnread(cur, library.Sources{}); len(got) != 0 {
		t.Errorf("без отдельных файлов: %v", got)
	}
	if got := prunedUnread(nil, library.Sources{Files: []string{keep}}); got != nil {
		t.Errorf("пустое осталось пустым: %v", got)
	}
}

// Журнал: пропуски по видам и причинам, без имён файлов; необычная причина — с текстом ошибки без пути.
func TestSkippedReasons(t *testing.T) {
	sk := []library.Skip{
		{Path: "/x/Иванова-1.jpg", Why: library.Why(errors.New("формат не поддерживается"), heicBytes())},
		{Path: "/x/Иванова-2.jpg", Why: library.Why(errors.New("формат не поддерживается"), heicBytes())},
		{Path: "/x/c.png", Why: "слишком большое (больше 250 Мп)"},
	}
	got := skippedReasons(sk)
	want := "2 файла .jpg (внутри HEIC, а не JPEG), 1 файл .png (слишком большое (больше 250 Мп))"
	if got != want {
		t.Errorf("\n%s\nждали\n%s", got, want)
	}
	if strings.Contains(got, "Иванова") || strings.Contains(got, "Paint") {
		t.Errorf("имя файла или совет в журнале: %s", got)
	}
	if got := skippedReasons(nil); got != "" {
		t.Errorf("пусто: %q", got)
	}
	long := skippedReasons([]library.Skip{{Path: "a.jpg", Why: strings.Repeat("я", 500) + "\nвторая строка"}})
	if strings.ContainsAny(long, "\n") || len([]rune(long)) > 200 {
		t.Errorf("длинная причина: %q", long)
	}
}
