package share

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg" // регистрирует разбор заголовка JPEG для image.DecodeConfig
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"

	"github.com/IliaSA101/sosninaalbums/api/albums"
	"github.com/IliaSA101/sosninaalbums/internal/export"
)

// BuildManifest собирает манифест из страниц, которые записал export.RunWeb в dir: sha256, байты и
// px каждого файла берутся из самого файла (заголовок JPEG читается без декодирования). cover — в
// книге есть обложка: первая страница front, последняя back. Пути в манифест не попадают.
// Размер файла и числа проверяются до чтения, ошибка — с числами (design §3.3 п. 4). ctx — остановка.
func BuildManifest(ctx context.Context, dir string, pages []export.WebPage, cover bool, spec export.WebSpec, title, generator string, lim albums.Limits) (*albums.Manifest, error) {
	lim = lim.OrDefault()
	if len(pages) == 0 || len(pages)%2 != 0 {
		return nil, fmt.Errorf("в книге %d страниц: нужно чётное число, не меньше двух", len(pages))
	}
	if len(pages) > lim.Pages {
		return nil, fmt.Errorf("в книге %d страниц, сервис принимает не больше %d", len(pages), lim.Pages)
	}
	tw, th := spec.Trimmed()
	if !(tw >= albums.MinPageMM && tw <= albums.MaxPageMM && th >= albums.MinPageMM && th <= albums.MaxPageMM) {
		return nil, fmt.Errorf("страница после обреза %v×%v мм: допустимо %d…%d мм", tw, th, albums.MinPageMM, albums.MaxPageMM)
	}
	gen, err := cleanGenerator(generator)
	if err != nil {
		return nil, err
	}
	m := &albums.Manifest{
		Schema:    albums.SchemaVersion,
		Generator: gen,
		Title:     CleanTitle(title, lim.TitleLen),
		Book:      albums.Book{Cover: cover, PageMM: albums.Size{W: tw, H: th}},
		Pages:     make([]albums.Page, len(pages)),
	}
	var total int64
	for i, wp := range pages {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		pg := albums.Page{N: i + 1, Role: albums.RolePage, Blank: wp.Blank}
		if cover && i == 0 {
			pg.Role = albums.RoleFront
		} else if cover && i == len(pages)-1 {
			pg.Role = albums.RoleBack
		}
		if !wp.Blank {
			fi, err := readPage(dir, wp.Name, lim.PageBytes)
			if err != nil {
				return nil, fmt.Errorf("страница %d: %w", pg.N, err)
			}
			pg.Type, pg.SHA256, pg.Bytes, pg.W, pg.H = albums.TypeJPEG, fi.sha, fi.bytes, fi.w, fi.h
			if total += fi.bytes; total > lim.AlbumBytes {
				return nil, fmt.Errorf("страницы занимают %s, сервис принимает не больше %s", sizeText(total, lim.AlbumBytes), sizeText(lim.AlbumBytes, lim.AlbumBytes))
			}
		}
		m.Pages[i] = pg
	}
	return m, nil
}

type pageFile struct {
	sha   string
	bytes int64
	w, h  int
}

// readPage читает страницу name из dir: обычный файл (ссылка — отказ), не больше limit байт,
// JPEG с размером 1…albums.MaxSide px. Имя — только имя файла, без пути.
func readPage(dir, name string, limit int64) (pageFile, error) {
	if name == "" || name != filepath.Base(name) || name == "." || name == ".." {
		return pageFile{}, fmt.Errorf("имя файла %q не годится", name)
	}
	p := filepath.Join(dir, name)
	lst, err := os.Lstat(p)
	if err != nil {
		return pageFile{}, errors.New("файл страницы не найден")
	}
	if !lst.Mode().IsRegular() {
		return pageFile{}, errors.New("файл страницы — не обычный файл")
	}
	if lst.Size() > limit {
		return pageFile{}, fmt.Errorf("файл %s больше предела %s", sizeText(lst.Size(), limit), sizeText(limit, limit))
	}
	f, err := os.Open(p)
	if err != nil {
		return pageFile{}, errors.New("файл страницы не открывается")
	}
	defer func() { _ = f.Close() }()
	// Тот же файл, что видел Lstat: подмена ссылкой между проверкой и открытием не проходит.
	if st, err := f.Stat(); err != nil || !st.Mode().IsRegular() || st.Size() > limit || !os.SameFile(lst, st) {
		return pageFile{}, errors.New("файл страницы изменился при чтении")
	}
	h := sha256.New()
	n, err := io.Copy(h, io.LimitReader(f, limit+1))
	if err != nil {
		return pageFile{}, errors.New("файл страницы не читается")
	}
	if n == 0 || n > limit {
		return pageFile{}, fmt.Errorf("файл %s: пустой или больше предела %s", sizeText(n, limit), sizeText(limit, limit))
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return pageFile{}, errors.New("файл страницы не читается")
	}
	cfg, format, err := image.DecodeConfig(io.LimitReader(f, limit))
	if err != nil || format != "jpeg" {
		return pageFile{}, errors.New("файл страницы — не JPEG")
	}
	if cfg.Width < 1 || cfg.Height < 1 || cfg.Width > albums.MaxSide || cfg.Height > albums.MaxSide {
		return pageFile{}, fmt.Errorf("размер страницы %d×%d px вне 1…%d", cfg.Width, cfg.Height, albums.MaxSide)
	}
	return pageFile{sha: hex.EncodeToString(h.Sum(nil)), bytes: n, w: cfg.Width, h: cfg.Height}, nil
}

// cleanGenerator — обозначение программы для манифеста: только печатные знаки, не длиннее
// albums.MaxGenerator. Подмены нет: пустое после очистки — ошибка вызывающего (версия не передана).
func cleanGenerator(s string) (string, error) {
	var b strings.Builder
	n := 0
	for _, r := range strings.ToValidUTF8(s, "") {
		if !unicode.IsPrint(r) || n >= albums.MaxGenerator {
			continue
		}
		b.WriteRune(r)
		n++
	}
	g := strings.TrimSpace(b.String())
	if g == "" {
		return "", errors.New("не указано, какая программа собрала альбом")
	}
	return g, nil
}

// sizeText — размер для отказа с числами: в КБ, пока предел unit меньше мегабайта, иначе в МБ
// (округление вверх, чтобы «больше предела» не выглядело равным пределу).
func sizeText(n, unit int64) string {
	if unit < 1<<20 {
		return strconv.FormatInt((n+1023)>>10, 10) + " КБ"
	}
	return strconv.FormatInt((n+(1<<20)-1)>>20, 10) + " МБ"
}
