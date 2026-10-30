package album

// Печать отпечатка (дополнения T4-9 после аудита): печать ставит только выдача отпечатка (SaveNew,
// SaveUID при чужой печати, сбор); открытие и обычная запись переносят её как есть. Сверка с путём —
// при выгрузке и показе ссылки (ShareIdent, SnapshotAt, NeedUID): копия в Проводнике, файл с
// другого пути, без печати и с негодной печатью — запись пути чужая, выгрузка выдаёт новый.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// copyFile — копия файла байт в байт (Проводник).
func copyFile(t *testing.T, from, to string) {
	t.Helper()
	data, err := os.ReadFile(from)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(to, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// openFile — Editor над файлом path, как его открывает App (Load, NewEditor).
func openFile(t *testing.T, path string) *Editor {
	t.Helper()
	q, err := salbum.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	e, err := NewEditor(q, path)
	if err != nil {
		t.Fatal(err)
	}
	return e
}

// fileTag — поле uidAt в файле.
func fileTag(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var f salbum.FileProject
	if err := json.Unmarshal(data, &f); err != nil {
		t.Fatal(err)
	}
	return f.UIDAt
}

// foreignAt — альбом открыт как есть (отпечаток не сменился), но запись о ссылке его пути ему
// чужая: ShareIdent без отпечатка, снимок без печати, NeedUID просит запись. Открытие — не правка.
func foreignAt(t *testing.T, e *Editor, uid string) {
	t.Helper()
	path, own := e.ShareIdent()
	if own != "" {
		t.Fatalf("печать не этого пути, а запись своя: %q", own)
	}
	if _, raw := e.Ident(); raw != uid {
		t.Fatalf("открытие сменило отпечаток: %q, в файле %q", raw, uid)
	}
	if _, q, _, sealed := e.SnapshotAt(); sealed || q.UID != uid {
		t.Fatalf("снимок выгрузки: печать %v, отпечаток %q", sealed, q.UID)
	}
	if need, err := e.NeedUID(path, uid); !need || err != nil {
		t.Fatalf("NeedUID: %v %v", need, err)
	}
	if len(e.undo) != 0 || e.dirty || e.rev != 0 {
		t.Fatalf("открытие — правка: шагов %d, dirty %v, rev %d", len(e.undo), e.dirty, e.rev)
	}
}

// ownAt — запись о ссылке пути альбома своя: ShareIdent и снимок с отпечатком uid, записи не нужно.
func ownAt(t *testing.T, e *Editor, uid string) {
	t.Helper()
	path, own := e.ShareIdent()
	if own != uid {
		t.Fatalf("запись своего альбома чужая: %q, ждали %q", own, uid)
	}
	if _, q, _, sealed := e.SnapshotAt(); !sealed || q.UID != uid {
		t.Fatalf("снимок выгрузки: печать %v, отпечаток %q", sealed, q.UID)
	}
	if need, err := e.NeedUID(path, uid); need || err != nil {
		t.Fatalf("NeedUID: %v %v", need, err)
	}
}

// pin — выдача при выгрузке (SaveUID): новый отпечаток, не из old, с печатью path в файле.
func pin(t *testing.T, e *Editor, old ...string) string {
	t.Helper()
	path, uid := e.Ident()
	nu, err := e.SaveUID(path, uid)
	if err != nil {
		t.Fatal(err)
	}
	for _, o := range append(old, uid) {
		if nu == o {
			t.Fatalf("SaveUID оставил отпечаток %q", nu)
		}
	}
	if fileTag(t, path) != model.UIDTag(nu, path) {
		t.Fatalf("печать в файле %q — не пути %s", fileTag(t, path), path)
	}
	ownAt(t, e, nu)
	return nu
}

// newAlbum — альбом, выданный «Сохранить как» по пути path (SaveNew): отпечаток с печатью path.
func newAlbum(t *testing.T, path string) (*Editor, string) {
	t.Helper()
	e, err := NewEditor(model.New(), "")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := e.SaveNew(path); err != nil {
		t.Fatal(err)
	}
	if fileTag(t, path) != model.UIDTag(e.p.UID, path) {
		t.Fatalf("SaveNew: печать %q — не пути", fileTag(t, path))
	}
	ownAt(t, e, e.p.UID)
	return e, e.p.UID
}

// Сценарий аудита: шаблон копируют каждому клиенту в одну папку X. Копия открыта — отпечаток
// шаблона как есть, но запись X чужая; выгрузка Иванова — новый отпечаток с печатью X, после
// перезапуска запись своя; Петрову — снова копия шаблона в X: запись Иванова ему чужая и после
// записи без правок. Две копии шаблона рядом — разные отпечатки. Сам шаблон свою запись держит.
func TestEditorSealTemplateCopies(t *testing.T) {
	dir := testkit.TempDir(t)
	tpl := filepath.Join(dir, "Шаблон.salbum")
	_, tu := newAlbum(t, tpl)
	ownAt(t, openFile(t, tpl), tu)
	x := filepath.Join(dir, "work", "Альбом.salbum")
	if err := os.MkdirAll(filepath.Dir(x), 0o755); err != nil {
		t.Fatal(err)
	}
	copyFile(t, tpl, x)
	ivan := openFile(t, x)
	foreignAt(t, ivan, tu)
	u1 := pin(t, ivan, tu) // выгрузка Иванова: запись X/u1
	ownAt(t, openFile(t, x), u1)

	copyFile(t, tpl, x) // архив Иванова унесли, Петрову — снова шаблон
	petr := openFile(t, x)
	foreignAt(t, petr, tu)
	if err := petr.Save(""); err != nil { // смена альбома или выход: запись без правок
		t.Fatal(err)
	}
	foreignAt(t, openFile(t, x), tu)
	u2 := pin(t, openFile(t, x), tu, u1)

	y := filepath.Join(dir, "work", "Альбом 2.salbum")
	copyFile(t, tpl, y)
	sid := openFile(t, y)
	foreignAt(t, sid, tu)
	pin(t, sid, tu, u1, u2)
	ownAt(t, openFile(t, x), u2)
}

// Регрессия аудита: альбом на USB-диске (E:), диск однажды получил другую букву (F:). Открыли
// там, сохранили без правок и с правками (смена альбома, автосохранение, выход): отпечаток и
// печать в файле прежние, запись F: альбому чужая. Диск снова E: — запись E: своя, ссылка видна.
// То же — облачная папка на двух ПК, переименование папки туда и обратно.
func TestEditorSealOtherPathKeeps(t *testing.T) {
	dir := testkit.TempDir(t)
	e := filepath.Join(dir, "E", "Иванов", "a.salbum")
	_, u := newAlbum(t, e)
	tag := fileTag(t, e)
	f := filepath.Join(dir, "F", "Иванов", "a.salbum")
	if err := os.MkdirAll(filepath.Join(dir, "F"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Dir(e), filepath.Dir(f)); err != nil {
		t.Fatal(err)
	}
	ed := openFile(t, f)
	foreignAt(t, ed, u)
	if err := ed.Save(""); err != nil { // без правок
		t.Fatal(err)
	}
	st := ed.p.Settings
	st.Gap++
	ed.SetSettings(st)
	if err := ed.Save(""); err != nil { // с правкой
		t.Fatal(err)
	}
	if err := ed.WriteCopy(filepath.Join(dir, "draft.salbum")); err != nil { // черновик
		t.Fatal(err)
	}
	if fileUID(t, f) != u || fileTag(t, f) != tag || fileTag(t, filepath.Join(dir, "draft.salbum")) != tag {
		t.Fatalf("запись по другому пути сменила отпечаток или печать: %q %q", fileUID(t, f), fileTag(t, f))
	}
	if _, own := ed.ShareIdent(); own != "" {
		t.Fatalf("после записи по F: запись пути своя: %q", own)
	}
	if err := os.Rename(filepath.Dir(f), filepath.Dir(e)); err != nil {
		t.Fatal(err)
	}
	back := openFile(t, e)
	ownAt(t, back, u)
	if back.p.Settings.Gap != st.Gap {
		t.Fatal("правка с F: потерялась")
	}
	if err := back.Save(""); err != nil || fileTag(t, e) != tag {
		t.Fatalf("Save по своему пути сменил печать: %q, %v", fileTag(t, e), err)
	}
}

// Файл ранней сборки 0.32 (отпечаток без печати) и файл, записанный без выдачи (salbum.Save):
// отпечаток как есть, запись пути чужая; выгрузка — новый. Регистр пути на Linux — другой путь
// (paths.Fold), на Windows и macOS — тот же; «.» в пути печать не меняет (Clean).
func TestEditorSealMovedAndBare(t *testing.T) {
	dir := testkit.TempDir(t)
	a := filepath.Join(dir, "a.salbum")
	_, u := newAlbum(t, a)
	tag := fileTag(t, a)
	data, err := os.ReadFile(a)
	if err != nil {
		t.Fatal(err)
	}
	bare := strings.Replace(string(data), `"uidAt": "`+tag+`",`, "", 1)
	if bare == string(data) {
		t.Fatal("печати нет в файле")
	}
	if err := os.WriteFile(a, []byte(bare), 0o644); err != nil {
		t.Fatal(err)
	}
	foreignAt(t, openFile(t, a), u)
	pin(t, openFile(t, a), u)

	p := model.New()
	p.UID = model.NewUID()
	b := filepath.Join(dir, "b.salbum")
	if err := salbum.Save(p, b); err != nil || fileTag(t, b) != "" {
		t.Fatalf("обычная запись поставила печать %q (%v)", fileTag(t, b), err)
	}
	foreignAt(t, openFile(t, b), p.UID)

	upper := filepath.Join(dir, "A.salbum")
	sameCase := model.UIDTag(u, upper) == tag
	if sameCase != foldsCase() {
		t.Errorf("регистр пути: печать совпала %v", sameCase)
	}
	if model.UIDTag(u, filepath.Join(dir, ".", "a.salbum")) != tag {
		t.Error("путь с «.» — другая печать: ключ записи о ссылке их сводит (Clean)")
	}
}

// foldsCase — пути без учёта регистра (Windows, macOS: paths.Fold).
func foldsCase() bool { return paths.Fold("A") == "a" }

// Печать в файле — чужие данные: негодная (регистр, длина, не hex), печать другого отпечатка или
// другого пути — отпечаток как есть, запись пути чужая; Load не падает. Негодная, положенная в
// проект мимо Load, — тоже не своя.
func TestEditorSealBadTag(t *testing.T) {
	dir := testkit.TempDir(t)
	path := filepath.Join(dir, "a.salbum")
	_, uid := newAlbum(t, path)
	p := &model.Project{UID: uid}
	good := model.UIDTag(p.UID, path)
	if fileTag(t, path) != good || !model.GoodUIDTag(good) {
		t.Fatalf("печать в файле %q, ждали %q", fileTag(t, path), good)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{
		strings.ToUpper(good), good[:15], good + "0", strings.Repeat("g", 16), "", `\"},\"x\":\"`,
		model.UIDTag(model.NewUID(), path), model.UIDTag(p.UID, filepath.Join(dir, "b.salbum")),
	} {
		if err := os.WriteFile(path, []byte(strings.Replace(string(data), good, bad, 1)), 0o644); err != nil {
			t.Fatal(err)
		}
		foreignAt(t, openFile(t, path), p.UID)
	}
	q := model.New()
	q.UID = uid
	q.SetUIDAt(strings.ToUpper(good))
	n, err := NewEditor(q, path)
	if err != nil {
		t.Fatal(err)
	}
	if _, own := n.ShareIdent(); own != "" {
		t.Fatalf("негодная печать мимо Load признана: %q", own)
	}
	if model.UIDTag("", path) != "" || model.UIDTag(p.UID, "") != "" || model.UIDTag("bad", path) != "" {
		t.Error("печать пустого или негодного")
	}
	if (&model.Project{UID: p.UID}).Sealed(path) {
		t.Error("проект без печати — Sealed")
	}
}

// Круг Save → Load: печать пути выдачи переносится как есть, Encode без пути — тоже; Save в
// другой путь тем же проектом (Save, не SaveNew) печать не меняет. Restore копии из папки копий
// этого альбома (байты файла X) — запись своя, и после Ctrl+Z и Ctrl+Y; копия старой выдачи
// (другой отпечаток с печатью X) — своя; копия чужого файла — отпечаток копии как есть, запись
// чужая, Ctrl+Z — снова своя; копия без отпечатка — новый, чужая. Альбом без файла — как есть.
func TestEditorSealRoundTripRestore(t *testing.T) {
	dir := testkit.TempDir(t)
	x := filepath.Join(dir, "x.salbum")
	e, u := newAlbum(t, x)
	tag := fileTag(t, x)
	q, err := salbum.Load(x)
	if err != nil || q.UIDAt() != tag || !q.Sealed(x) || q.Sealed(filepath.Join(dir, "y.salbum")) {
		t.Fatalf("Load: печать %q, %v", q.UIDAt(), err)
	}
	if data, err := salbum.Encode(q, dir); err != nil || !strings.Contains(string(data), tag) {
		t.Fatalf("Encode потерял печать: %v", err)
	}
	other := filepath.Join(dir, "sub", "x.salbum")
	if err := os.MkdirAll(filepath.Dir(other), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := e.Save(other); err != nil || fileTag(t, other) != tag {
		t.Fatalf("Save в другой путь: печать %q, %v", fileTag(t, other), err)
	}
	if err := e.Save(x); err != nil {
		t.Fatal(err)
	}
	bakDir := filepath.Join(dir, "backups")
	if err := os.MkdirAll(bakDir, 0o755); err != nil {
		t.Fatal(err)
	}
	bak := filepath.Join(bakDir, "2026-10-29.salbum")
	copyFile(t, x, bak)
	gap := e.p.Settings.Gap
	st := e.p.Settings
	st.Gap++
	e.SetSettings(st)
	if err := e.Save(""); err != nil {
		t.Fatal(err)
	}
	restore := func(file string) {
		t.Helper()
		cp, err := LoadBase(file, dir)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := e.Restore(cp); err != nil {
			t.Fatal(err)
		}
	}
	restore(bak)
	if e.p.Settings.Gap != gap {
		t.Fatal("копия не восстановлена")
	}
	ownAt(t, e, u)
	e.Undo()
	ownAt(t, e, u)
	e.Redo()
	ownAt(t, e, u)

	// Тот же отпечаток с печатью другого пути (копия переносили): своя печать альбома не теряется.
	moved := e.p.Clone()
	moved.SetUIDAt(model.UIDTag(u, other))
	moved.Settings.Gap += 3
	if _, err := e.Restore(moved); err != nil {
		t.Fatal(err)
	}
	ownAt(t, e, u)
	e.Undo()

	// Старая выдача этого пути: отпечаток копии с печатью X — запись X ему своя (в файле его нет).
	old := model.New()
	old.UID = model.NewUID()
	old.SetUIDAt(model.UIDTag(old.UID, x))
	oldFile := filepath.Join(bakDir, "old.salbum")
	if err := salbum.Save(old, oldFile); err != nil {
		t.Fatal(err)
	}
	restore(oldFile)
	if _, own := e.ShareIdent(); own != old.UID {
		t.Fatalf("копия старой выдачи этого пути: %q", own)
	}
	e.Undo()
	ownAt(t, e, u)

	y := filepath.Join(dir, "y.salbum")
	_, yu := newAlbum(t, y)
	restore(y)
	if e.p.UID != yu {
		t.Fatalf("копия чужого файла: отпечаток %q, ждали как есть %q", e.p.UID, yu)
	}
	if _, own := e.ShareIdent(); own != "" {
		t.Fatalf("копия чужого файла — запись X своя: %q", own)
	}
	e.Undo()
	ownAt(t, e, u)

	bare := filepath.Join(bakDir, "bare.salbum")
	if err := salbum.Save(model.New(), bare); err != nil {
		t.Fatal(err)
	}
	restore(bare)
	if _, own := e.ShareIdent(); own != "" || e.p.UID == u || !model.GoodUID(e.p.UID) {
		t.Fatalf("копия без отпечатка: %q, своя %q", e.p.UID, own)
	}
	e.Undo()
	ownAt(t, e, u)

	cp2, err := LoadBase(bak, "")
	if err != nil {
		t.Fatal(err)
	}
	n, err := NewEditor(cp2, "") // копия открыта отдельно (openBackup): файла нет — отпечаток как есть
	if err != nil {
		t.Fatal(err)
	}
	if n.p.UID != u {
		t.Fatalf("копия без файла: %q", n.p.UID)
	}
	if _, own := n.ShareIdent(); own != "" {
		t.Fatalf("альбом без файла — своя запись: %q", own)
	}
}
