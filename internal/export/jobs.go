package export

import (
	"context"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"strconv"

	"github.com/IliaSA101/sosninaalbums/internal/render"
)

// Job — что записать из разворота Index (с 0): целиком (NN.jpg) и/или левую (NN-1.jpg),
// правую (NN-2.jpg) страницу. Страницы режутся из одного рендера на холсте из двух половин.
type Job struct {
	Index              int
	Whole, Left, Right bool
}

// PageName — имя файла страницы: page 1 — левая (05-1.jpg), 2 — правая (05-2.jpg).
// Номер и ширина — как у FileName, от числа разворотов альбома.
func PageName(i, total, page int) string {
	return fmt.Sprintf("%0*d-%d.jpg", max(2, len(strconv.Itoa(total))), i+1, page)
}

// Names — имена файлов, которые запишет задание.
func (j Job) Names(total int) []string {
	var out []string
	if j.Whole {
		out = append(out, FileName(j.Index, total))
	}
	if j.Left {
		out = append(out, PageName(j.Index, total, 1))
	}
	if j.Right {
		out = append(out, PageName(j.Index, total, 2))
	}
	return out
}

// RunJobs пишет файлы по заданиям в dir; spreads — весь альбом (номер и ширина имени — от его
// длины, а не от числа заданий). progress вызывается после каждого готового задания.
func RunJobs(ctx context.Context, spreads []render.Spread, jobs []Job, opt render.Options, dir string, progress func(done, total int)) error {
	if err := CheckCanvas(opt); err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	total := len(spreads)
	for n, j := range jobs {
		if err := ctx.Err(); err != nil {
			return err
		}
		if j.Index < 0 || j.Index >= total {
			return fmt.Errorf("разворот %d: нет в альбоме", j.Index+1)
		}
		if err := runJob(spreads[j.Index], j, opt, total, dir); err != nil {
			return fmt.Errorf("разворот %d: %w", j.Index+1, err)
		}
		progress(n+1, len(jobs))
	}
	return nil
}

func runJob(s render.Spread, j Job, opt render.Options, total int, dir string) error {
	if j.Whole {
		img, err := render.Render(s, opt)
		if err != nil {
			return err
		}
		if err := saveJPEG(img, filepath.Join(dir, FileName(j.Index, total)), opt); err != nil {
			return err
		}
	}
	if !j.Left && !j.Right {
		return nil
	}
	so := opt
	so.Split = true
	if err := CheckCanvas(so); err != nil {
		return err
	}
	img, err := render.Render(s, so)
	if err != nil {
		return err
	}
	half := img.Bounds().Dx() / 2
	for page, on := range [2]bool{j.Left, j.Right} {
		if !on {
			continue
		}
		r := image.Rect(page*half, 0, (page+1)*half, img.Bounds().Dy())
		if err := saveJPEG(img.SubImage(r), filepath.Join(dir, PageName(j.Index, total, page+1)), opt); err != nil {
			return err
		}
	}
	return nil
}
