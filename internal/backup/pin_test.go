package backup

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
	"time"
)

// Закреплённая копия переживает и чистку Keep при записи, и Prune по возрасту и по числу папок
// (0.33-149-19): ForceKeep её бы не уберёг, фоновая Prune — удалила.
func TestBackupPinSurvivesPrune(t *testing.T) {
	s, c := newStore(t)
	p := albumFile(t, `{"n":"агент"}`)
	key := Key(p)
	pinned, err := s.ForcePin(p, key)
	if err != nil || pinned == "" {
		t.Fatalf("ForcePin: %q, %v", pinned, err)
	}
	for i := range Keep + 5 { // чистка Keep: закреплённая самая давняя, но остаётся
		c.t = c.t.Add(time.Minute)
		rewrite(t, p, fmt.Sprintf(`{"n":%d}`, i))
		if err := s.Force(p, key); err != nil {
			t.Fatal(err)
		}
	}
	got := names(t, s, key)
	if !slices.Contains(got, pinned) || len(got) != Keep {
		t.Fatalf("после чистки Keep: %v (закреплена %s)", got, pinned)
	}
	// Prune по возрасту: все копии старше MaxAge, свежая другого альбома двигает «сейчас».
	other := albumFile(t, `{"o":1}`)
	c.t = c.t.Add(MaxAge + 24*time.Hour)
	if err := s.Force(other, Key(other)+"-x"); err != nil {
		t.Fatal(err)
	}
	if err := s.Prune(); err != nil {
		t.Fatal(err)
	}
	if got := names(t, s, key); !slices.Equal(got, []string{pinned}) {
		t.Fatalf("после Prune по возрасту: %v, нужна одна %s", got, pinned)
	}
	// Prune по числу папок: альбом с закреплённой копией целиком не удаляется.
	for i := range MaxDirs + 2 {
		c.t = c.t.Add(time.Minute)
		q := albumFile(t, fmt.Sprintf(`{"q":%d}`, i))
		if err := s.Force(q, fmt.Sprintf("a%d-00000000", i)); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.Prune(); err != nil {
		t.Fatal(err)
	}
	if got := names(t, s, key); !slices.Equal(got, []string{pinned}) {
		t.Fatalf("после Prune по числу папок: %v", got)
	}
	ents, _ := os.ReadDir(s.Root)
	if len(ents) > MaxDirs+1 {
		t.Fatalf("папок %d: лишние не убраны", len(ents))
	}
}

// ForcePin: совпавшая с новейшей копия закрепляется без дубля; нет файла — "" без ошибки;
// нет хранилища, плохой ключ, диск без записи — ошибка; права файла 0o600.
func TestBackupForcePinEdges(t *testing.T) {
	s, _ := newStore(t)
	p := albumFile(t, `{"a":1}`)
	key := Key(p)
	first, err := s.ForcePin(p, key)
	if err != nil || first == "" {
		t.Fatal(first, err)
	}
	again, err := s.ForcePin(p, key) // тот же файл: та же копия, не вторая
	if err != nil || again != first || len(names(t, s, key)) != 1 {
		t.Fatalf("повтор: %q, %v, %v", again, err, names(t, s, key))
	}
	if runtime.GOOS != "windows" {
		if st, err := os.Stat(filepath.Join(s.Root, key, first)); err != nil || st.Mode().Perm() != 0o600 {
			t.Errorf("права копии: %v, %v", st, err)
		}
	}
	if n, err := s.ForcePin(filepath.Join(filepath.Dir(p), "нет.salbum"), key); n != "" || err != nil {
		t.Fatalf("нет файла: %q, %v", n, err)
	}
	if _, err := s.ForcePin(p, "../x"); err == nil {
		t.Fatal("ключ с «..» принят")
	}
	var nilStore *Store
	if _, err := nilStore.ForcePin(p, key); !errors.Is(err, ErrNoStore) {
		t.Fatalf("nil: %v", err)
	}
	if _, err := New("").ForcePin(p, key); !errors.Is(err, ErrNoStore) {
		t.Fatalf("Root пуст: %v", err)
	}
	if err := nilStore.Pin(key, first); !errors.Is(err, ErrNoStore) {
		t.Fatalf("Pin nil: %v", err)
	}
	if err := s.Pin(key, "чужое.txt"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Pin с чужим именем: %v", err)
	}
	if err := s.Pin(key, "20200101-000000.salbum"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Pin без файла: %v", err)
	}
	// Корень копий — файл, а не папка: копия не записана, ошибка наружу.
	bad := New(filepath.Join(filepath.Dir(p), "корень"))
	if err := os.WriteFile(bad.Root, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if n, err := bad.ForcePin(p, key); n != "" || err == nil {
		t.Fatalf("диск не принял: %q, %v", n, err)
	}
}

// Несколько закреплённых копий одного альбома (разные запуски одного ключа) живут вместе.
func TestBackupPinMany(t *testing.T) {
	s, c := newStore(t)
	p := albumFile(t, `{}`)
	key := Key(p)
	var pins []string
	for i := range 3 {
		rewrite(t, p, fmt.Sprintf(`{"n":%d}`, i))
		n, err := s.ForcePin(p, key)
		if err != nil {
			t.Fatal(err)
		}
		pins = append(pins, n)
		c.t = c.t.Add(time.Minute)
	}
	for i := range Keep + 3 {
		rewrite(t, p, fmt.Sprintf(`{"w":%d}`, i))
		if err := s.Force(p, key); err != nil {
			t.Fatal(err)
		}
		c.t = c.t.Add(time.Minute)
	}
	got := names(t, s, key)
	for _, n := range pins {
		if !slices.Contains(got, n) {
			t.Fatalf("закреплённая %s пропала: %v", n, got)
		}
	}
}
