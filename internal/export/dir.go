package export

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/render"
)

var (
	oldSpreadRe = regexp.MustCompile(`(?i)^spread_(\d+)\.jpg$`)         // экспорт v0.2: наш, метки в нём нет
	numberedRe  = regexp.MustCompile(`(?i)^(\d{2,})(?:-([12]))?\.jpg$`) // 01.jpg, 01-1.jpg…, 0001.jpg: наш только с COM-меткой
)

// tmpRe — остаток WriteFileAtomic: 05.jpg.<число>.tmp; с префиксом плана — 1-05.jpg.<число>.tmp
// (081 №9). Префикс — только плана (из профиля типографии, QuoteMeta), не любое число: чужой
// 2024-05.jpg.7.tmp не наш. Без префикса плана остаток без префикса тоже наш.
func tmpRe(prefix string) *regexp.Regexp {
	return regexp.MustCompile(`(?i)^(?:` + regexp.QuoteMeta(prefix) + `)?(\d{2,})(?:-[12])?\.jpg\.\d+\.tmp$`)
}

// DirFile — файл папки экспорта: Spread — номер разворота из имени (с 1, ширина не важна),
// Page — 0 (разворот целиком), 1 или 2 (страница).
type DirFile struct {
	Name         string
	Spread, Page int
}

// PlanDir разбирает папку экспорта: ours — файлы прежнего экспорта программы
// (их можно заменить), foreign — NN.jpg / NN-1.jpg без метки, а также папка или ссылка
// с таким именем (под них экспорт отказывает).
// 00.jpg/000.jpg (обложка из шаблона типографии) и прочие файлы не трогаем никогда.
func PlanDir(dir string) (ours, foreign []DirFile, err error) { return PlanDirPrefix(dir, "") }

// PlanDirPrefix — PlanDir для имён с префиксом плана (081 №9): при «1-» разворот — 1-NN.jpg,
// 1-00.jpg — обложка (как 00.jpg); NN.jpg и spread_NN.jpg этой выгрузке не свои и не мешают.
func PlanDirPrefix(dir, prefix string) (ours, foreign []DirFile, err error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil, err
	}
	numbered := numberedRe
	if prefix != "" {
		numbered = regexp.MustCompile(`(?i)^` + regexp.QuoteMeta(prefix) + `(\d{2,})(?:-([12]))?\.jpg$`)
	}
	for _, e := range entries {
		name := e.Name()
		regular := e.Type().IsRegular() // папка и ссылка — не файл экспорта, но и не пустое место
		if m := oldSpreadRe.FindStringSubmatch(name); m != nil && prefix == "" {
			n, _ := strconv.Atoi(m[1])
			f := DirFile{Name: name, Spread: n}
			if regular {
				ours = append(ours, f)
			} else {
				foreign = append(foreign, f)
			}
			continue
		}
		m := numbered.FindStringSubmatch(name)
		if m == nil || strings.Trim(m[1], "0") == "" {
			continue // не номер разворота или обложка
		}
		n, _ := strconv.Atoi(m[1]) // огромный номер → MaxInt (Atoi при переполнении): ни одной цели не совпадёт, но файл учтён
		page, _ := strconv.Atoi(m[2])
		f := DirFile{Name: name, Spread: n, Page: page}
		if regular && IsOurs(filepath.Join(dir, name)) {
			ours = append(ours, f)
		} else {
			foreign = append(foreign, f)
		}
	}
	return ours, foreign, nil
}

// HitFiles — файлы из списка, которые задевает план.
func HitFiles(pl Plan, files []DirFile) []DirFile {
	var out []DirFile
	for _, f := range files {
		if pl.hitsFile(f) {
			out = append(out, f)
		}
	}
	return out
}

// removeFiles удаляет перечисленные файлы папки (имена из PlanDir): иначе при меньшем
// числе разворотов остались бы «хвосты» старого экспорта. Возвращает ошибку первого сбоя.
// Метка проверяется заново: пока шёл рендер, под нашим именем мог появиться чужой файл.
func removeFiles(dir string, names []string) error {
	for _, n := range names {
		if n != filepath.Base(n) {
			return fmt.Errorf("недопустимое имя файла %q", n)
		}
		p := filepath.Join(dir, n)
		if !oldSpreadRe.MatchString(n) && !IsOurs(p) { // spread_NN.jpg (v0.2) метки не имеет
			continue
		}
		if err := os.Remove(p); err != nil {
			return err
		}
	}
	return nil
}

// StaleNames — имена из hit, которых новый экспорт не перезапишет. Имена сравниваются без
// учёта регистра: на Windows 01.JPG и 01.jpg — один файл, и удалить его значило бы стереть новый.
func StaleNames(hit []DirFile, written []string) []string {
	var out []string
	for _, f := range hit {
		keep := false
		for _, w := range written {
			if strings.EqualFold(f.Name, w) {
				keep = true
				break
			}
		}
		if !keep {
			out = append(out, f.Name)
		}
	}
	return out
}

// RunPlan пишет файлы плана и только после успеха удаляет заменённые старые (stale):
// сбой посреди выгрузки не должен стоить прежнего экспорта.
func RunPlan(ctx context.Context, spreads []render.Spread, pl Plan, opt render.Options, dir string,
	stale []string, progress func(done, total int)) error {
	removeStaleTmp(dir, pl.Prefix, time.Now())
	if err := runCoverPlan(ctx, spreads, pl, opt, dir, progress); err != nil {
		return err
	}
	// «Остановить» во время кодирования последнего листа: RenderCtx уже вернулся, а прежние
	// файлы удалять нельзя — выгрузка не завершена (080-11).
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := removeFiles(dir, stale); err != nil {
		return fmt.Errorf("не удалось удалить прежние файлы экспорта: %w", err)
	}
	return nil
}

// tmpMaxAge — возраст, после которого .tmp экспорта считается остатком сбоя, а не записью,
// которая идёт сейчас (другой копией программы).
const tmpMaxAge = time.Hour

// removeStaleTmp убирает остатки WriteFileAtomic (05.jpg.<число>.tmp) старше tmpMaxAge:
// имя случайное, поэтому после сбоев они копились бы в папке. Ошибки не мешают экспорту.
// Остаток записи обложки (00.jpg.<число>.tmp, 1-00.jpg.<число>.tmp) не трогаем: он не отличим от чужого файла типографии.
func removeStaleTmp(dir, prefix string, now time.Time) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	re := tmpRe(prefix)
	for _, e := range entries {
		m := re.FindStringSubmatch(e.Name())
		if m == nil || strings.Trim(m[1], "0") == "" || !e.Type().IsRegular() {
			continue
		}
		if info, err := e.Info(); err == nil && now.Sub(info.ModTime()) > tmpMaxAge {
			_ = os.Remove(filepath.Join(dir, e.Name()))
		}
	}
}
