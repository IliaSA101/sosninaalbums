package svgcut

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ResolveSrc возвращает полный путь файла rel внутри каталога исходников root. Пустой root или rel,
// абсолютный путь, «..» и выход по символической ссылке — отказ: рецепт не должен читать чужое.
func ResolveSrc(root, rel string) (string, error) {
	if strings.TrimSpace(root) == "" {
		return "", errors.New("svgcut: пустой каталог исходников (-src)")
	}
	if strings.TrimSpace(rel) == "" {
		return "", errors.New("svgcut: пустой путь исходника")
	}
	if !filepath.IsLocal(rel) {
		return "", fmt.Errorf("svgcut: исходник %q должен лежать внутри -src (без .. и абсолютного пути)", rel)
	}
	rootReal, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", fmt.Errorf("svgcut: каталог исходников: %w", err)
	}
	full, err := filepath.EvalSymlinks(filepath.Join(root, rel))
	if err != nil {
		return "", fmt.Errorf("svgcut: исходник: %w", err)
	}
	r, err := filepath.Rel(rootReal, full)
	if err != nil || !filepath.IsLocal(r) {
		return "", fmt.Errorf("svgcut: исходник %q выходит за пределы -src", rel)
	}
	return full, nil
}

// ReadSrc читает SVG из root/rel: размер проверяется до чтения, файл должен быть обычным.
func ReadSrc(root, rel string) ([]byte, error) {
	full, err := ResolveSrc(root, rel)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(full)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !st.Mode().IsRegular() {
		return nil, fmt.Errorf("svgcut: %q не обычный файл", rel)
	}
	if st.Size() > MaxSVGBytes {
		return nil, fmt.Errorf("svgcut: %q больше %d МБ", rel, MaxSVGBytes>>20)
	}
	return ReadLimited(io.Reader(f))
}
