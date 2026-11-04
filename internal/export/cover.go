package export

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/render"
)

// CoverName — файл обложки: 00.jpg при любом числе разворотов (номер 00 FileName не занимает).
const CoverName = "00.jpg"

// CoverJob — обложка к выгрузке: лист и параметры рендера (холст — размер листа, не разворота).
// Name — имя файла (00.jpg, 1-00.jpg): ставит Plan.WithCover по префиксу плана.
type CoverJob struct {
	Spread render.Spread
	Opt    render.Options
	Name   string
}

// name — имя файла обложки; задание мимо плана (Name "") — CoverName.
func (j *CoverJob) name() string {
	if j.Name == "" {
		return CoverName
	}
	return j.Name
}

// NewCoverJob собирает задание обложки из вида печати (Editor.Snapshot): без направляющих
// и разметки, поля 030 подставлены. nil — обложки нет. dpi и quality уже зажаты ClampOptions.
func NewCoverJob(p *model.Project, v model.View, dpi, quality int) *CoverJob {
	c := v.Cover
	if c == nil {
		return nil
	}
	s := coverSpread(p, c, printable(c.Sheet)) // заготовки (131) не печатаются
	// Холст рендера — две половины PageW; у обложки PageW = W/2, а шов закрывает плашка.
	return &CoverJob{Spread: s, Opt: render.Options{PageW: c.Sheet.W / 2, PageH: c.Sheet.H, DPI: dpi, Quality: quality}, Name: CoverName}
}

// CoverScene — сцена обложки для кадра пипетки: как NewCoverJob (с плашкой корешка), но
// заготовки (131) на месте — холст их показывает. Обложки нет — false.
func CoverScene(p *model.Project, v model.View) (render.Spread, bool) {
	c := v.Cover
	if c == nil {
		return render.Spread{}, false
	}
	return coverSpread(p, c, c.Sheet), true
}

// coverSpread — лист обложки sheet (вид c.Sheet, возможно без заготовок) и плашка корешка.
func coverSpread(p *model.Project, c *model.CoverView, sheet model.SpreadView) render.Spread {
	s := fromSheet(p, sheet)
	if c.SpineBg != "" { // плашка во весь корешок: поверх фонов сторонок, под фото корешка, местами и текстами
		for _, z := range c.Zones {
			if z.Kind == "spine" && z.Rect.W > 0 && z.Rect.H > 0 {
				s.Plates = append(s.Plates, render.Plate{Rect: z.Rect, Color: render.ParseHex(c.SpineBg)})
			}
		}
	}
	return s
}

// WithCover добавляет обложку в план: её имя (по префиксу плана), вопрос о замене и прогресс.
// Задание копируется: имя в одном плане не меняет обложку другого.
func (pl Plan) WithCover(j *CoverJob) Plan {
	if j == nil {
		pl.Cover = nil
		return pl
	}
	c := *j
	c.Name = pl.CoverName()
	pl.Cover = &c
	return pl
}

// runCover рисует и пишет файл обложки (00.jpg; с префиксом плана — 1-00.jpg).
func runCover(ctx context.Context, j *CoverJob, dir string) error {
	if err := CheckCanvas(j.Opt); err != nil {
		return fmt.Errorf("обложка: %w", err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	img, err := render.RenderCtx(ctx, j.Spread, j.Opt)
	if err != nil {
		return fmt.Errorf("обложка: %w", err)
	}
	if err := saveJPEG(img, filepath.Join(dir, j.name()), j.Opt); err != nil {
		return fmt.Errorf("обложка: %w", err)
	}
	return nil
}

// runCoverPlan пишет обложку (первый шаг) и задания разворотов с общим прогрессом.
func runCoverPlan(ctx context.Context, spreads []render.Spread, pl Plan, opt render.Options, dir string, progress func(done, total int)) error {
	total, shift := len(pl.Jobs), 0
	if pl.Cover != nil {
		total, shift = total+1, 1
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := runCover(ctx, pl.Cover, dir); err != nil {
			return err
		}
		progress(1, total)
	}
	if len(pl.Jobs) == 0 {
		return nil
	}
	return RunJobs(ctx, spreads, pl.Jobs, opt, dir, func(done, _ int) { progress(done+shift, total) })
}

// AddCoverFile добавляет в списки папки существующий 00.jpg, если план пишет обложку: наш
// (с меткой) заменяется после вопроса, чужой — отказ, как у файлов разворотов. Не обычный
// файл (ссылка, папка) — тоже чужой: ссылку или папку не заменяем молча. PlanDir 00.jpg
// не отдаёт: без обложки в плане этот файл не трогаем никогда (шаблон типографии).
func AddCoverFile(pl Plan, dir string, ours, foreign []DirFile) ([]DirFile, []DirFile) {
	if pl.Cover == nil {
		return ours, foreign
	}
	name := pl.CoverName() // 1-00.jpg у плана с префиксом
	path := filepath.Join(dir, name)
	st, err := os.Lstat(path)
	if err != nil {
		return ours, foreign
	}
	if st.Mode().IsRegular() && IsOurs(path) {
		return append(ours, DirFile{Name: name}), foreign
	}
	return ours, append(foreign, DirFile{Name: name})
}

// BlankCover — обложка в виде печати пуста: ни фото, ни мест, ни текстов (заготовки не в счёт), ни фото фона, ни
// узора, ни стикера, ни своего цвета страниц и плашки корешка.
func BlankCover(v model.View) bool {
	c := v.Cover
	if c == nil {
		return false
	}
	s := printable(c.Sheet)
	return len(s.Photos) == 0 && len(s.Texts) == 0 && len(s.Captions) == 0 && !shownBackdrop(s) && !shownPattern(s) &&
		!shownSticker(s) && len(s.Places) == 0 && s.BgSet == [2]string{} && c.SpineBg == ""
}

// BlankWarningCover — вопрос о пустых разворотах и пустой обложке (cover — обложка пуста
// и идёт в выгрузку); "" — пустого нет. white — фон альбома белый.
func BlankWarningCover(nums []int, cover, white bool) string {
	if !cover {
		return BlankWarning(nums, white)
	}
	tail := "белыми"
	if !white {
		tail = "с фоном альбома"
	}
	switch len(nums) {
	case 0:
		if white {
			tail = "белой"
		}
		return fmt.Sprintf("Обложка пустая — выгрузить %s?", tail)
	case 1:
		return fmt.Sprintf("Обложка и разворот %d пустые — выгрузить %s?", nums[0], tail)
	}
	return fmt.Sprintf("Обложка и развороты %s пустые — выгрузить %s?", numList(nums), tail)
}
