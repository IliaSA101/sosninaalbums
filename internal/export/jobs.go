package export

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/IliaSA101/sosninaalbums/internal/render"
)

// Job — что записать из разворота Index (с 0): целиком (NN.jpg) и/или левую (NN-1.jpg),
// правую (NN-2.jpg) страницу. Страницы режутся из одного рендера на холсте из двух половин.
// Prefix — перед каждым именем (1-05.jpg у Фабрики Фотокниги, 081 №9); ставит Plan.WithPrefix.
type Job struct {
	Index              int
	Whole, Left, Right bool
	Prefix             string
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
		out = append(out, j.Prefix+FileName(j.Index, total))
	}
	if j.Left {
		out = append(out, j.Prefix+PageName(j.Index, total, 1))
	}
	if j.Right {
		out = append(out, j.Prefix+PageName(j.Index, total, 2))
	}
	return out
}

// RunJobs пишет файлы по заданиям в dir; spreads — весь альбом (номер и ширина имени — от его
// длины, а не от числа заданий). progress вызывается после каждого готового задания из
// горутины записи; к возврату RunJobs все вызовы завершены. Рендер следующего листа идёт
// параллельно с записью предыдущего (pipe.go).
func RunJobs(ctx context.Context, spreads []render.Spread, jobs []Job, opt render.Options, dir string, progress func(done, total int)) error {
	if err := CheckCanvas(opt); err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return runPipe(ctx, spreads, jobs, opt, dir, progress)
}
