package app

// Правки агента 0.35 (154): фон альбома (set_background, to: album). Правило то же, что у
// agents_edit34.go: rev и срок сверяются под замком Editor (SetAlbumBgIf), один вызов — один шаг
// отмены, холостой — без шага и тоста, «Недавние» не пишутся.

import (
	"context"
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/app/agents"
)

// agentsAlbumBg — Host.Bg с in.Album: фон альбома из каталога (узор или фото), как «Фон альбома» в окне
// «Фоны», но без вопроса 115: свои фоны каталога снимаются, листы с фото клиента остаются (Skipped), если
// их нет в листах согласия in.Spreads (только с Replace).
func (a *App) agentsAlbumBg(ctx context.Context, ed *album.Editor, in agents.BgReq) (res agents.BgRes, err error) {
	prev := ed.View()
	s, err := a.decorStore()
	if err != nil {
		return res, agentHostErr(err)
	}
	bg, err := s.Background(in.Item)
	if err != nil {
		return res, agentHostErr(agentDecorErr(err))
	}
	var consent []string
	if in.Replace {
		consent = in.Spreads
	}
	v, out, err := ed.SetAlbumBgIf(model.BgSource{Pattern: bg.Pattern, Photo: bg.Photo}, in.Pages && !prev.Settings.Sheet, consent,
		album.Expect{Rev: in.Rev, Ctx: ctx})
	if err != nil {
		return res, agentHostErr(agentErr(err))
	}
	res = agents.BgRes{Album: agents.AlbumRef{Key: in.Key, Rev: v.Rev}, Skipped: out.Skipped, Replaced: out.Replaced > 0,
		Cleared: out.Cleared, AlbumBg: !reflect.DeepEqual(prev.AlbumBg, v.AlbumBg)}
	for _, sv := range v.Spreads {
		if spreadChanged(prev, v, sv.ID) {
			res.Changed++
		}
		if sv.NoAlbumBg && !slices.Contains(out.Skipped, sv.ID) {
			res.Hidden = append(res.Hidden, sv.ID)
		}
	}
	if v.Rev != prev.Rev { // фон альбома — шаг и в альбоме без разворотов
		a.changed(v)
		a.agentEdited(AgentEdited{Client: agentToastClient(in.Client), Text: albumBgToast(v, out.Cleared), Rev: v.Rev})
	}
	a.agentDraftStep(in.Key, in.Rev, res.Album.Rev) // черновик агента остаётся нетронутым
	return res, nil
}

// albumBgToast — «Фон альбома поставлен» либо, если сняты свои фоны, «Фон альбома — теперь и на 3
// разворотах: 3, 7, 18» (слова — ownDoneText человека).
func albumBgToast(v model.View, cleared []string) string {
	if len(cleared) == 0 {
		return "Фон альбома поставлен"
	}
	n := len(cleared)
	unit := plural(n, "развороте", "разворотах", "разворотах")
	if v.Settings.Sheet {
		unit = plural(n, "листе", "листах", "листах")
	}
	return fmt.Sprintf("Фон альбома — теперь и на %d %s: %s", n, unit, numsText(spreadNums(v, cleared)))
}

// spreadNums — номера с 1 листов по их ID в виде v, по возрастанию; чужие ID пропускаются.
func spreadNums(v model.View, ids []string) []int {
	var nums []int
	for i, sv := range v.Spreads {
		if slices.Contains(ids, sv.ID) {
			nums = append(nums, i+1)
		}
	}
	return nums
}

// numsMax — сколько номеров перечислять в тосте и вопросе про фон альбома; остальные — «и ещё N»
// (OWN_NUMS_MAX человека).
const numsMax = 12

// numsText — «3, 5-8, 18»: подряд идущие три и больше — диапазоном; больше numsMax номеров — первые и
// «, и ещё N» (formatRanges и ownNumsText человека).
func numsText(nums []int) string {
	nums = slices.Compact(slices.Sorted(slices.Values(nums)))
	head, rest := nums, 0
	if len(nums) > numsMax {
		head, rest = nums[:numsMax], len(nums)-numsMax
	}
	var parts []string
	for i := 0; i < len(head); {
		j := i
		for j+1 < len(head) && head[j+1] == head[j]+1 {
			j++
		}
		if j-i >= 2 {
			parts = append(parts, strconv.Itoa(head[i])+"-"+strconv.Itoa(head[j]))
		} else {
			for _, n := range head[i : j+1] {
				parts = append(parts, strconv.Itoa(n))
			}
		}
		i = j + 1
	}
	out := strings.Join(parts, ", ")
	if rest > 0 {
		out += ", и ещё " + strconv.Itoa(rest)
	}
	return out
}
