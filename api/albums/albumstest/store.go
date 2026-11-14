package albumstest

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/IliaSA101/sosninaalbums/api/albums"
)

// blobs — хранилище картинок страниц по (id альбома, sha256). Имена файлов собирает только сервер
// из проверенных id (CheckID) и sha256 (64 hex): знаков пути от клиента в них нет.
type blobs interface {
	put(id, sha, ext string, data []byte) error
	get(id, sha, ext string) ([]byte, bool)
	drop(id string)
}

func newBlobs(dir string) (blobs, error) {
	if dir == "" {
		return &memBlobs{m: map[string][]byte{}}, nil
	}
	if !filepath.IsAbs(dir) {
		return nil, errors.New("albumstest: Dir должен быть абсолютным путём")
	}
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		return nil, errors.New("albumstest: Dir не существует или не папка")
	}
	root := filepath.Join(dir, "albums-fake")
	if err := os.MkdirAll(root, 0o700); err != nil {
		return nil, fmt.Errorf("albumstest: %w", err)
	}
	return &diskBlobs{root: root}, nil
}

type memBlobs struct {
	mu sync.Mutex
	m  map[string][]byte
}

func (b *memBlobs) put(id, sha, _ string, data []byte) error {
	b.mu.Lock()
	b.m[id+"/"+sha] = append([]byte(nil), data...)
	b.mu.Unlock()
	return nil
}

func (b *memBlobs) get(id, sha, _ string) ([]byte, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	d, ok := b.m[id+"/"+sha]
	return d, ok
}

func (b *memBlobs) drop(id string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for k := range b.m {
		if len(k) > len(id) && k[:len(id)+1] == id+"/" {
			delete(b.m, k)
		}
	}
}

type diskBlobs struct{ root string }

func (b *diskBlobs) path(id, sha, ext string) (string, bool) {
	if albums.CheckID(id) != nil || len(sha) != 64 || (ext != "jpg" && ext != "webp") {
		return "", false
	}
	return filepath.Join(b.root, id, sha+"."+ext), true
}

func (b *diskBlobs) put(id, sha, ext string, data []byte) error {
	p, ok := b.path(id, sha, ext)
	if !ok {
		return errors.New("albumstest: негодное имя")
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o600)
}

func (b *diskBlobs) get(id, sha, ext string) ([]byte, bool) {
	p, ok := b.path(id, sha, ext)
	if !ok {
		return nil, false
	}
	d, err := os.ReadFile(p)
	return d, err == nil
}

// drop удаляет папку альбома: id уже проверен CheckID и найден в таблице, путь — root/id.
func (b *diskBlobs) drop(id string) {
	if albums.CheckID(id) != nil {
		return
	}
	_ = os.RemoveAll(filepath.Join(b.root, id))
}
