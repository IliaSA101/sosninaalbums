//go:build ignore

// qa-bigalbum строит большой тестовый .salbum для QA окна «Открываем альбом» (073): N разворотов
// по K фото, раскладка «Авто».
//
//	go run scripts/qa-bigalbum.go -out .testphotos/big200.salbum -photos .testphotos/stock -spreads 200 -per 16
//
// Фото берутся из папки по кругу со сдвигом: составы разворотов почти не повторяются, иначе кэш
// раскладок сделал бы открытие мгновенным. Загрузчик держит каждый файл в одном месте проекта,
// поэтому на каждое место создаётся своя ссылка на файл (жёсткая, при отказе — копия) в папке
// -links; по умолчанию она вне .testphotos, чтобы не раздувать обычный набор QA.
// Проект собирается публичным API Editor (AddFolder, NewSpread) и пишется через Save.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/library"
)

func main() {
	out := flag.String("out", ".testphotos/big200.salbum", "куда писать .salbum")
	dir := flag.String("photos", ".testphotos/stock", "папка с фото (по кругу)")
	links := flag.String("links", ".testphotos-big/qa-bigalbum", "папка ссылок на фото альбома")
	n := flag.Int("spreads", 200, "разворотов")
	per := flag.Int("per", 16, "фото на разворот")
	flag.Parse()
	if err := run(*out, *dir, *links, *n, *per); err != nil {
		fmt.Fprintln(os.Stderr, "qa-bigalbum:", err)
		os.Exit(1)
	}
}

func run(out, dir, links string, n, per int) error {
	if n < 1 || n > model.MaxSpreads || per < 1 || per > model.MaxPhotos {
		return fmt.Errorf("разворотов 1..%d, фото на разворот 1..%d", model.MaxSpreads, model.MaxPhotos)
	}
	src, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	if links, err = filepath.Abs(links); err != nil {
		return err
	}
	if out, err = filepath.Abs(out); err != nil {
		return err
	}
	base, names, err := readPhotos(src)
	if err != nil {
		return err
	}
	if len(base) < per {
		return fmt.Errorf("в %s фото %d, а на разворот нужно %d", src, len(base), per)
	}
	if err := os.MkdirAll(links, 0o750); err != nil {
		return err
	}
	t0 := time.Now()
	e, err := album.NewEditor(model.New(), "")
	if err != nil {
		return err
	}
	if _, _, err := e.AddFolder(links); err != nil {
		return err
	}
	x := uint64(n*1000 + per)
	for i := range n {
		spread := make([]model.PhotoInfo, 0, per)
		used := map[int]bool{}
		for len(spread) < per {
			x = x*6364136223846793005 + 1442695040888963407
			k := int(x >> 33 % uint64(len(base)))
			if used[k] {
				continue
			}
			used[k] = true
			ph := base[k]
			ph.Path = filepath.Join(links, fmt.Sprintf("s%03d-%02d-%s", i+1, len(spread)+1, names[k]))
			if err := link(filepath.Join(src, names[k]), ph.Path); err != nil {
				return err
			}
			ph.ID = library.IDFor(ph.Path)
			spread = append(spread, ph)
		}
		if _, err := e.NewSpread(-1, spread); err != nil {
			return fmt.Errorf("разворот %d: %w", i+1, err)
		}
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o750); err != nil {
		return err
	}
	if err := e.Save(out); err != nil {
		return err
	}
	// Круг: файл читается обратно тем же загрузчиком, что и программа.
	p, err := salbum.Load(out)
	if err != nil {
		return fmt.Errorf("проверочное чтение: %w", err)
	}
	if len(p.Spreads) != n {
		return fmt.Errorf("после чтения разворотов %d, ждали %d", len(p.Spreads), n)
	}
	for i, s := range p.Spreads {
		if len(s.Photos) != per {
			return fmt.Errorf("после чтения на развороте %d фото %d, ждали %d", i+1, len(s.Photos), per)
		}
	}
	st, err := os.Stat(out)
	if err != nil {
		return err
	}
	fmt.Printf("%s: %d разворотов по %d фото, %d КБ, сборка %s\nфото: %s\n",
		out, n, per, st.Size()/1024, time.Since(t0).Round(time.Millisecond), links)
	return nil
}

// readPhotos читает размеры всех JPEG/PNG в папке (без подпапок) по порядку имён.
func readPhotos(dir string) ([]model.PhotoInfo, []string, error) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil, err
	}
	var names []string
	for _, en := range ents {
		switch strings.ToLower(filepath.Ext(en.Name())) {
		case ".jpg", ".jpeg", ".png":
			if !en.IsDir() {
				names = append(names, en.Name())
			}
		}
	}
	sort.Strings(names)
	list := make([]model.PhotoInfo, 0, len(names))
	for _, name := range names {
		ph, err := library.ReadInfo(filepath.Join(dir, name))
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %w", name, err)
		}
		list = append(list, model.PhotoInfo{W: ph.W, H: ph.H, Size: ph.Size})
	}
	return list, names, nil
}

// link кладёт в dst ссылку на src (жёсткую, на другом томе — копию); готовый файл не трогает.
func link(src, dst string) error {
	if _, err := os.Stat(dst); err == nil {
		return nil
	}
	if os.Link(src, dst) == nil {
		return nil
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, in); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
