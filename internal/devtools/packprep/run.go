package packprep

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/devtools/svgcut"
)

// Имена служебного в папке набора.
const (
	ManifestSrcName = "manifest.src.json"
	LicenseName     = "LICENSE.txt"
	CheckDir        = "_check"
	ReportName      = "report.json"
	TmpDir          = ".tmp"
)

// Config — запуск. Src и Out обязательны: пустое значение — ошибка, а не текущая папка (в Windows —
// корень диска, root-guard). Пути CLI приводит к абсолютным и проверяет paths.CheckLocalPath.
type Config struct {
	Src    string            // каталог исходников; src рецепта — только внутри
	Out    string            // папка набора; временные файлы — Out/.tmp
	Chrome string            // браузер для svg (PACKPREP_CHROME)
	Rembg  string            // команда rembg для листов стикеров (SHEETCUT_REMBG)
	Render svgcut.RenderFunc // подмена растра SVG (тесты); nil — Chrome
}

// runner — состояние одного прогона.
type runner struct {
	cfg   Config
	rec   *Recipe
	rep   *Report
	outs  []*Output
	names map[string]bool // файлы предметов в нижнем регистре: повтор — ошибка
	edges []edgeBlock     // углы растровых источников для edges.png
	seen  map[string]bool // источники, чьи углы уже сняты
	order int
}

// Run готовит папку набора по рецепту. Ошибка — только негодная конфигурация или сбой записи; брак
// предметов (нет файла, сбой браузера, бюджет, семейство) копится в Report.Errors, а остальные
// предметы всё равно обрабатываются: один прогон — все провалы, лист проверки есть и при них.
func Run(cfg Config, rec *Recipe) (*Report, error) {
	if strings.TrimSpace(cfg.Src) == "" || strings.TrimSpace(cfg.Out) == "" {
		return nil, errors.New("packprep: -src и -out обязательны: пустое значение не заменяется текущей папкой")
	}
	if rec == nil {
		return nil, errors.New("packprep: нет рецепта")
	}
	src, err := filepath.Abs(cfg.Src)
	if err != nil {
		return nil, err
	}
	out, err := filepath.Abs(cfg.Out)
	if err != nil {
		return nil, err
	}
	if filepath.Dir(out) == out || filepath.Dir(src) == src {
		return nil, errors.New("packprep: -src и -out не могут быть корнем диска")
	}
	if st, err := os.Stat(src); err != nil || !st.IsDir() {
		return nil, fmt.Errorf("packprep: -src %q не папка", cfg.Src)
	}
	cfg.Src, cfg.Out = src, out
	if err := prepareOut(out); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Join(out, TmpDir), 0o755); err != nil {
		return nil, err
	}
	r := &runner{cfg: cfg, rec: rec, names: map[string]bool{}, seen: map[string]bool{}}
	r.rep = &Report{ID: rec.ID, Version: rec.Version}
	if cfg.Render == nil && cfg.Chrome != "" {
		r.cfg.Render = svgcut.Chrome{Cmd: cfg.Chrome, Tmp: filepath.Join(out, TmpDir)}.Render
	}
	r.rep.Errors = append(r.rep.Errors, checkFamilies(rec)...)
	for i, it := range rec.Items {
		r.item(i, it)
	}
	if err := r.finish(); err != nil {
		return r.rep, err
	}
	_ = os.RemoveAll(filepath.Join(out, TmpDir))
	return r.rep, nil
}

// prepareOut создаёт папку набора и убирает следы прошлого прогона: только то, что записал сам
// packprep (файлы из прежнего manifest.src.json, _check, .tmp, служебные файлы). Чужое в непустой
// папке без манифеста — отказ: -out не должна оказаться чьей-то рабочей папкой.
func prepareOut(out string) error {
	entries, err := os.ReadDir(out)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return os.MkdirAll(out, 0o755)
	case err != nil:
		return err
	}
	manifest := filepath.Join(out, ManifestSrcName)
	prev, perr := os.ReadFile(manifest)
	if len(entries) > 0 && perr != nil {
		only := true
		for _, e := range entries {
			if e.Name() != TmpDir && e.Name() != CheckDir {
				only = false
			}
		}
		if !only {
			return fmt.Errorf("packprep: -out %q не пуста и не похожа на папку набора (нет %s)", out, ManifestSrcName)
		}
	}
	if perr == nil {
		var m struct {
			Items []struct {
				File string `json:"file"`
			} `json:"items"`
		}
		if json.Unmarshal(prev, &m) == nil {
			for _, it := range m.Items {
				if ValidFileName(it.File) {
					_ = os.Remove(filepath.Join(out, filepath.FromSlash(it.File)))
				}
			}
		}
	}
	for _, name := range []string{ManifestSrcName, LicenseName, CheckDir, TmpDir} {
		if err := os.RemoveAll(filepath.Join(out, name)); err != nil {
			return err
		}
	}
	return nil
}

// finish — порядок, номера, бюджет, подсказки, манифест и лист проверки.
func (r *runner) finish() error {
	arrange(r.outs)
	r.rep.Items = r.outs
	markSimilar(r.outs)
	markForm(r.outs)
	r.rep.budget()
	r.rep.collect()
	if len(r.outs) == 0 {
		r.rep.fail("в наборе нет ни одного предмета")
	}
	r.rep.OK = len(r.rep.Errors) == 0
	if err := os.MkdirAll(filepath.Join(r.cfg.Out, CheckDir), 0o755); err != nil {
		return err
	}
	if err := r.writeCheck(); err != nil {
		return err
	}
	if len(r.outs) == 0 {
		return nil
	}
	return r.writeManifest()
}
