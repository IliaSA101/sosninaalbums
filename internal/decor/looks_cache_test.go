package decor

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Кэш разобранных макетов Look (looks_cache.go, 0.28-131-210).

func lookFileOf(cfg, key string) string {
	return filepath.Join(cfg, RootName, kindDirs[KindAlbum], strings.TrimPrefix(key, libPrefix)+".json")
}

func savedLook(t *testing.T, s *Store, name string) Item {
	t.Helper()
	l := lookOf("", "")
	l.Steps[0].Name = name
	it, err := s.SaveLook("mine", name, l)
	if err != nil {
		t.Fatal(err)
	}
	return it
}

// Попадание: файл подменён той же длины и тем же временем правки — Look отдаёт прежний разбор, то есть
// файл не читался; сменилось время или размер — файл читается заново; файла нет — ErrNotFound.
func TestLookCacheHitAndStat(t *testing.T) {
	s, cfg := openTemp(t)
	it := savedLook(t, s, "Первый")
	path := lookFileOf(cfg, it.ID)
	first, err := s.Look(it.ID)
	if err != nil || len(first.Steps) != 3 {
		t.Fatalf("Look: %v %+v", err, first)
	}
	st, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(strings.Repeat("{", int(st.Size()))), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, st.ModTime(), st.ModTime()); err != nil {
		t.Fatal(err)
	}
	if got, err := s.Look(it.ID); err != nil || !reflect.DeepEqual(got, first) {
		t.Fatalf("попадание в кэш: %v %+v", err, got)
	}
	later := st.ModTime().Add(time.Hour)
	if err := os.Chtimes(path, later, later); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Look(it.ID); err == nil {
		t.Error("время правки сменилось, а битый файл не замечен")
	}
	// Тот же ключ, файл вернули прежний (другой размер, чем у битого) — снова читается и кэшируется.
	if err := os.WriteFile(path, mustMarshalLook(t, first), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, err := s.Look(it.ID); err != nil || len(got.Steps) != 3 {
		t.Fatalf("после починки: %v", err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Look(it.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("файла нет: %v", err)
	}
	if _, ok := s.lookTpl.m[it.ID]; ok {
		t.Error("запись удалённого файла осталась в кэше")
	}
	// Ключ не из индекса и пустой — ErrNotFound без обращения к кэшу.
	for _, id := range []string{"", "my:zz", "my:00000000000000ff"} {
		if _, err := s.Look(id); !errors.Is(err, ErrNotFound) {
			t.Errorf("Look(%q): %v", id, err)
		}
	}
}

func mustMarshalLook(t *testing.T, l model.AlbumTpl) []byte {
	t.Helper()
	return lookBytes(t, l, "")
}

// Обновление набора: прежний ключ исчез (кэш его не держит), новый читается; SaveLook нового макета
// не портит записи прежнего.
func TestLookCacheAfterPackUpdateAndSave(t *testing.T) {
	s, _ := openTemp(t)
	bg := renderPNG(t, 40, 30)
	v1 := lookBytes(t, lookOf(libKey(bg), ""), "")
	m, src := testPack("wedding-layouts", 1, pit{"bg/a.png", bg}, pit{"looks/main.json", v1})
	install(t, s, m, src, nil)
	key1 := model.LookKey(v1)
	if l, err := s.Look(key1); err != nil || l.Steps[0].Name != "Титул" {
		t.Fatalf("v1: %v", err)
	}
	own := savedLook(t, s, "Свой")
	if _, err := s.Look(own.ID); err != nil {
		t.Fatal(err)
	}
	l2 := lookOf(libKey(bg), "")
	l2.Steps[0].Name = "Начало"
	v2 := lookBytes(t, l2, "")
	m2, src2 := testPack("wedding-layouts", 2, pit{"bg/a.png", bg}, pit{"looks/main.json", v2})
	st, err := s.StagePack(m2, src2, OriginFile, strings.Repeat("cd", 32))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.InstallPack(st, nil); err != nil {
		t.Fatal(err)
	}
	st.Close()
	if _, err := s.Look(key1); !errors.Is(err, ErrNotFound) {
		t.Errorf("прежний ключ после обновления: %v", err)
	}
	if l, err := s.Look(model.LookKey(v2)); err != nil || l.Steps[0].Name != "Начало" {
		t.Errorf("новый ключ: %v %+v", err, l)
	}
	// Скрытие и переименование файл не трогают — альбомы с макетом не ломаются, Look тот же.
	if err := s.Hide(KindAlbum, own.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.Rename(KindAlbum, own.ID, "Другое имя"); err != nil {
		t.Fatal(err)
	}
	if l, err := s.Look(own.ID); err != nil || l.Steps[0].Name != "Свой" {
		t.Errorf("свой после скрытия: %v", err)
	}
}

// Копия не портит кэш: правки результата (в том числе срезов и указателей) не видны следующему Look.
func TestLookCacheCopyIsolated(t *testing.T) {
	s, _ := openTemp(t)
	l := lookOf("", "")
	l.Cover = &model.CoverTpl{Bg: &model.TplBg{Color: "#112233"}, Back: &l.Steps[0].Tpl}
	it, err := s.SaveLook("mine", "Копия", l)
	if err != nil {
		t.Fatal(err)
	}
	pristine, err := s.Look(it.ID)
	if err != nil {
		t.Fatal(err)
	}
	want := string(mustMarshalLook(t, pristine)) // снимок байтами: не делит память с кэшем даже при ошибке копии
	scribble(reflect.ValueOf(&pristine).Elem())  // и первый результат (разбор) не делит память с кэшем
	for range 2 {                                // первый — разбор, второй — попадание: копия и там, и там
		got, err := s.Look(it.ID)
		if err != nil {
			t.Fatal(err)
		}
		scribble(reflect.ValueOf(&got).Elem())
		got.Steps = got.Steps[:1]
		if again, err := s.Look(it.ID); err != nil || string(mustMarshalLook(t, again)) != want {
			t.Fatalf("кэш испорчен правкой копии: %v", err)
		}
	}
}

// scribble портит всё дерево на месте: строки, числа, булевы — через указатели и срезы.
func scribble(v reflect.Value) {
	switch v.Kind() {
	case reflect.Pointer:
		if !v.IsNil() {
			scribble(v.Elem())
		}
	case reflect.Slice, reflect.Array:
		for i := range v.Len() {
			scribble(v.Index(i))
		}
	case reflect.Struct:
		for i := range v.NumField() {
			if f := v.Field(i); f.CanSet() {
				scribble(f)
			}
		}
	case reflect.String:
		v.SetString("!портчено")
	case reflect.Float32, reflect.Float64:
		v.SetFloat(99)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v.SetInt(99)
	case reflect.Bool:
		v.SetBool(!v.Bool())
	}
}

// fillAll заполняет значение ненулевым: указатели, срезы из двух элементов, словари. depth — предел
// на случай рекурсивного типа.
func fillAll(v reflect.Value, depth int) {
	if depth > 12 {
		return
	}
	switch v.Kind() {
	case reflect.Pointer:
		v.Set(reflect.New(v.Type().Elem()))
		fillAll(v.Elem(), depth+1)
	case reflect.Slice:
		v.Set(reflect.MakeSlice(v.Type(), 2, 2))
		for i := range 2 {
			fillAll(v.Index(i), depth+1)
		}
	case reflect.Map:
		v.Set(reflect.MakeMap(v.Type()))
		k, e := reflect.New(v.Type().Key()).Elem(), reflect.New(v.Type().Elem()).Elem()
		fillAll(k, depth+1)
		fillAll(e, depth+1)
		v.SetMapIndex(k, e)
	case reflect.Struct:
		for i := range v.NumField() {
			if f := v.Field(i); f.CanSet() {
				fillAll(f, depth+1)
			}
		}
	case reflect.String:
		v.SetString("s")
	case reflect.Float32, reflect.Float64:
		v.SetFloat(1.5)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v.SetInt(3)
	case reflect.Bool:
		v.SetBool(true)
	}
}

// disjoint: у копии нет ни одного общего с оригиналом указателя, среза и словаря; неэкспортируемых
// полей в дереве нет (их cloneLook не копирует).
func disjoint(t *testing.T, path string, a, b reflect.Value) {
	t.Helper()
	switch a.Kind() {
	case reflect.Pointer:
		if a.IsNil() != b.IsNil() || a.IsNil() {
			return
		}
		if a.Pointer() == b.Pointer() {
			t.Errorf("%s: общий указатель", path)
		}
		disjoint(t, path+"*", a.Elem(), b.Elem())
	case reflect.Slice:
		if a.Len() != b.Len() {
			t.Fatalf("%s: длина", path)
		}
		if a.Len() > 0 && a.Pointer() == b.Pointer() {
			t.Errorf("%s: общий срез", path)
		}
		for i := range a.Len() {
			disjoint(t, path+"[]", a.Index(i), b.Index(i))
		}
	case reflect.Map:
		if a.Len() > 0 && a.Pointer() == b.Pointer() {
			t.Errorf("%s: общий словарь", path)
		}
		for it := a.MapRange(); it.Next(); {
			disjoint(t, path+"{}", it.Value(), b.MapIndex(it.Key()))
		}
	case reflect.Struct:
		for i := range a.NumField() {
			if !a.Type().Field(i).IsExported() {
				t.Errorf("%s: неэкспортируемое поле %s", path, a.Type().Field(i).Name)
				continue
			}
			disjoint(t, path+"."+a.Type().Field(i).Name, a.Field(i), b.Field(i))
		}
	case reflect.Interface, reflect.Func, reflect.Chan, reflect.UnsafePointer:
		t.Errorf("%s: тип %s не копируется", path, a.Type())
	}
}

func TestCloneLookDeep(t *testing.T) {
	var src model.AlbumTpl
	fillAll(reflect.ValueOf(&src).Elem(), 0)
	got := cloneLook(src)
	if !reflect.DeepEqual(src, got) {
		t.Fatal("копия не равна оригиналу")
	}
	disjoint(t, "AlbumTpl", reflect.ValueOf(src), reflect.ValueOf(got))
	// Пустое и nil сохраняются как есть.
	e := cloneLook(model.AlbumTpl{Steps: []model.LookStep{}})
	if e.Steps == nil || len(e.Steps) != 0 || e.Cover != nil {
		t.Errorf("пустой макет: %+v", e)
	}
	if n := cloneLook(model.AlbumTpl{}); n.Steps != nil {
		t.Errorf("nil-срез стал непустым: %+v", n)
	}
}

// Предел: больше lookCacheMax макетов — лишние вытесняются (самый давний), сумма размеров в пределе.
func TestLookCacheBound(t *testing.T) {
	s, _ := openTemp(t)
	var ids []string
	for i := range lookCacheMax + 5 {
		ids = append(ids, savedLook(t, s, "М"+strings.Repeat("я", i+1)).ID)
	}
	for _, id := range ids {
		if _, err := s.Look(id); err != nil {
			t.Fatal(err)
		}
	}
	c := &s.lookTpl
	c.mu.Lock()
	n, sum := len(c.m), c.bytes
	var bySum int64 // сумма по оставшимся записям: remove обязан вычитать размер вытесненных
	for _, el := range c.m {
		bySum += el.Value.(*lookCacheEnt).size
	}
	_, oldest := c.m[ids[0]]
	_, newest := c.m[ids[len(ids)-1]]
	c.mu.Unlock()
	if n != lookCacheMax || oldest || !newest || sum <= 0 || sum > lookCacheBytes || sum != bySum {
		t.Errorf("в кэше %d записей, %d байт (по записям %d); самый давний %v, свежий %v", n, sum, bySum, oldest, newest)
	}
	// Файл больше предела суммы кэш не берёт.
	big := parsedLooks{}
	big.put("k", "p", fakeInfo{size: lookCacheBytes + 1}, model.AlbumTpl{})
	if len(big.m) != 0 {
		t.Error("файл больше предела суммы попал в кэш")
	}
}

// Запись привязана к пути из индекса: тот же ключ по другому пути — промах, запись снимается.
func TestLookCachePathChange(t *testing.T) {
	var c parsedLooks
	fi := fakeInfo{size: 10}
	c.put("k", "a.json", fi, model.AlbumTpl{V: model.LookV})
	if _, ok := c.get("k", "a.json", fi); !ok {
		t.Fatal("тот же путь — промах")
	}
	if _, ok := c.get("k", "b.json", fi); ok {
		t.Error("другой путь — попадание")
	}
	if len(c.m) != 0 || c.bytes != 0 {
		t.Errorf("запись по старому пути осталась: %d записей, %d байт", len(c.m), c.bytes)
	}
}

type fakeInfo struct {
	os.FileInfo
	size int64
}

func (f fakeInfo) Size() int64        { return f.size }
func (f fakeInfo) ModTime() time.Time { return time.Time{} }

// Гонка (-race): Look из нескольких горутин, пока файл подменяют и сохраняют новые макеты.
func TestLookCacheRace(t *testing.T) {
	s, cfg := openTemp(t)
	it := savedLook(t, s, "Гонка")
	path := lookFileOf(cfg, it.ID)
	good, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 60 {
				if l, err := s.Look(it.ID); err == nil {
					scribble(reflect.ValueOf(&l).Elem()) // правка копии не должна гоняться с другими
				}
			}
		}()
	}
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := range 30 {
			l := lookOf("", "")
			l.Steps[0].Name = "Новый" + strings.Repeat("я", i+1)
			if _, err := s.SaveLook("mine", "Новый", l); err != nil { // Fatal из горутины нельзя
				t.Error(err)
				return
			}
		}
	}()
	go func() {
		defer wg.Done()
		for i := range 30 {
			data := good
			if i%2 == 1 {
				data = []byte("{")
			}
			_ = os.WriteFile(path, data, 0o644)
			_ = os.Chtimes(path, time.Now().Add(time.Duration(i)*time.Second), time.Now().Add(time.Duration(i)*time.Second))
		}
	}()
	wg.Wait()
}

// Бенчмарк: Look без кэша (чтение и разбор файла) и с кэшем (стат и копия) на макете в 24 шага.
func benchLook(b *testing.B) (*Store, string) {
	b.Helper()
	s := Open(testkit.TempDir(b))
	l := lookOf("", "st")
	steps := make([]model.LookStep, 0, model.MaxLookSteps)
	for i := range model.MaxLookSteps {
		st := l.Steps[0]
		st.Name = "Шаг" + strings.Repeat("я", i)
		st.Tpl.Texts = []model.TplText{{Text: strings.Repeat("Текст ", 20)}}
		steps = append(steps, st)
	}
	l.Steps = steps
	it, err := s.SaveLook("mine", "Бенч", l)
	if err != nil {
		b.Fatal(err)
	}
	return s, it.ID
}

func BenchmarkLookUncached(b *testing.B) {
	s, id := benchLook(b)
	b.ResetTimer()
	for range b.N {
		if _, _, err := s.lookRaw(id); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkLookCached(b *testing.B) {
	s, id := benchLook(b)
	if _, err := s.Look(id); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for range b.N {
		if _, err := s.Look(id); err != nil {
			b.Fatal(err)
		}
	}
}
