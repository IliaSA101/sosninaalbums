package libpacks

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"slices"

	"github.com/IliaSA101/sosninaalbums/internal/decor"
	"github.com/IliaSA101/sosninaalbums/internal/decor/fetch"
	"github.com/IliaSA101/sosninaalbums/internal/decor/pack"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
)

// Одна дорога для всех операций сети (спека 137, design §1, §4): прежний сохранённый список →
// каталог → план → цикл установки → library:changed и checked.

// opKind — какая кнопка (или проверка при запуске) запустила ход.
type opKind int

const (
	opUpdate opKind = iota // «Обновить библиотеку»
	opPick                 // «Скачать» в строке
	opAll                  // «Скачать все»
	opStart                // проверка при запуске
	opDemo                 // «Скачать» в строке примера (без каталога)
)

// op — операция: id нужен «Скачать», offerOver > 0 — порог проверки при запуске.
type op struct {
	kind      opKind
	id        string
	offerOver int64
}

// plan — что ставить: want — id, допущенные в план (прочие строки каталога не трогаются и причин
// не дают); force — id, чей пропуск («удалён вами») не помеха: нажатый набор, его доноры и
// строки «Скачать все».
type plan struct {
	want, force map[string]bool
}

func newUpdate() Update {
	return Update{Skipped: []Skip{}, Failed: []Skip{}, why: map[string]string{}}
}

func idSet(ids ...string) map[string]bool {
	m := make(map[string]bool, len(ids))
	for _, id := range ids {
		m[id] = true
	}
	return m
}

func catIDs(cat fetch.Catalog) map[string]bool {
	m := make(map[string]bool, len(cat.Packs))
	for _, e := range cat.Packs {
		m[e.ID] = true
	}
	return m
}

// update — каталог → план → наборы по одному. offerOver > 0 и новое больше него — только
// предложение (Start.Offer), без скачивания. seq каталога пишется сразу после проверки подписи
// (меньший потом не примется), сохранённый список — следом; checked — после хода без обрыва.
func (s *Service) update(ctx context.Context, st *decor.Store, o op) (res Update, start Start, err error) {
	res = newUpdate()
	prog := s.progress()
	c := s.h.Fetcher()
	// Раздел, каким его видел пользователь: до нового каталога. «Скачать все» ставит ровно его
	// строки, новое сравнивается с ним.
	prev, shown, bad := s.beforeCatalog(st, o)
	prog("catalog", "", 0, 0)
	cat, err := s.Fresh(ctx) // seq, сохранённый список и пропущенные строки — один код с роликами
	if err != nil {
		logNetErr("проверка не удалась", err)
		return res, start, err
	}
	if bad && prev == nil {
		prev = catIDs(cat) // файл не читался: все наборы «новыми» не объявляем
	}
	have := haveOf(st)
	ix := indexOf(cat)
	pl, err := s.planOf(cat, ix, have, shown, o)
	if err != nil {
		return res, start, err
	}
	todo, need := s.netPlan(st, cat, pl, &res)
	defer func() {
		s.clearSkips(pl.force, &res)
		if res.Added+res.Updated > 0 {
			s.retireDemo(st) // пример уходит в том же замке, что установка донора
			s.h.Relink(st)
			s.emitChanged(res.titles...)
		}
		fresh := s.tally(cat, st, prev, &res)
		s.announce(cat) // объявлено пользователю: ходом, а не каталогом для роликов (140, 0.30-140-24)
		start.Updated, start.Fresh = res.Added+res.Updated, fresh
		if o.kind == opPick {
			s.pickResult(ix.byID[o.id], have, st, &res)
		}
	}()
	if o.kind == opStart && o.offerOver > 0 && need > o.offerOver {
		s.markChecked()
		start.Offer = Offer{Count: len(todo), Bytes: need}
		log.Printf("библиотека: проверка, новых %d (%d МБ) — предложено", len(todo), need>>20)
		return res, start, nil
	}
	if err = s.installAll(ctx, st, c, todo, &res, prog, decor.OriginNet); err != nil {
		return res, start, err
	}
	// Неудачи отдельных наборов ходу не мешали и проверку закрывают: иначе испорченный у
	// издателя архив качался бы при каждом запуске. «Скачать» одного набора checked не трогает.
	if o.kind != opPick {
		s.markChecked()
	}
	log.Printf("библиотека: итог — новых %d, обновлено %d, пропущено %d, не скачано %d",
		res.Added, res.Updated, len(res.Skipped), len(res.Failed))
	return res, start, nil
}

// beforeCatalog — что пользователю уже объявлено (prev: id наборов), строки раздела для
// «Скачать все» (shown) и признак «файл списка не читался». «Объявлено» — config.json
// (libraryNet.announced), а не сохранённый список: его двигает и каталог роликов (Fresh), и
// наборы тогда никогда бы не стали «новыми». Нет поля (старый файл) — состав сохранённого списка.
func (s *Service) beforeCatalog(st *decor.Store, o op) (prev map[string]bool, shown []string, bad bool) {
	n := prefs.LibraryNetOf(s.h.CfgDir())
	old, _, bad := s.cached()
	switch {
	case n.Announced != nil:
		prev = idSet(n.Announced...)
	case old != nil:
		prev = catIDs(*old)
	}
	if old != nil && o.kind == opAll {
		for _, r := range downloadable(availRows(*old, haveOf(st), n.Skip, s.h.Version())) {
			shown = append(shown, r.ID)
		}
	}
	return prev, shown, bad
}

// announce — весь свежий список объявлен: следующая проверка сравнивает с ним.
func (s *Service) announce(cat fetch.Catalog) {
	ids := make([]string, 0, len(cat.Packs))
	for _, e := range cat.Packs {
		ids = append(ids, e.ID)
	}
	prefs.UpdateLibraryNet(s.h.CfgDir(), func(n *prefs.LibraryNet) { n.Announced = ids })
}

// planOf — что ставить операции: «Обновить» и проверка при запуске — стоящее и недостающие
// дополнения; «Скачать» — состав нажатого (ErrGone, если его нет в свежем списке); «Скачать все» —
// объединение составов строк, показанных до операции.
func (s *Service) planOf(cat fetch.Catalog, ix catIndex, have map[string]decor.PackInfo, shown []string, o op) (plan, error) {
	skip, app := prefs.LibraryNetOf(s.h.CfgDir()).Skip, s.h.Version()
	switch o.kind {
	case opPick:
		e, ok := ix.byID[o.id]
		if !ok {
			return plan{}, ErrGone // список уже переписан: раздел перечитают
		}
		cl := ix.closure(cat, e.ID, have, skip, app)
		return plan{want: cl.set, force: idSet(append([]string{o.id}, cl.up...)...)}, nil
	case opAll:
		pl := plan{want: map[string]bool{}, force: map[string]bool{}}
		for _, id := range shown {
			if _, ok := ix.byID[id]; !ok {
				log.Printf("библиотека: строка %s ушла из списка — «Скачать все» её пропускает", id)
				continue
			}
			cl := ix.closure(cat, id, have, skip, app)
			pl.force[id] = true
			for _, up := range cl.up {
				pl.force[up] = true
			}
			for k := range cl.set {
				pl.want[k] = true
			}
		}
		return pl, nil
	}
	return plan{want: updateWant(cat, have)}, nil
}

// tally — итоги по свежему списку и библиотеке после хода: Available — строки «Скачать все»,
// Fresh — строки раздела без серой кнопки и «удалён вами», которых не было в прежнем списке.
func (s *Service) tally(cat fetch.Catalog, st *decor.Store, prev map[string]bool, res *Update) (fresh Offer) {
	skip := prefs.LibraryNetOf(s.h.CfgDir()).Skip
	have, app := haveOf(st), s.h.Version()
	_, all := shownPlan(cat, have, skip, app)
	res.Available = all.Count
	for _, r := range downloadable(availRows(cat, have, skip, app)) {
		if !prev[r.ID] {
			fresh.Count++
			fresh.Bytes += r.Bytes
		}
	}
	res.Fresh = fresh.Count
	return fresh
}

// clearSkips — «удалён вами» снимается с тех, кого эта операция поставила: отказ сети, ErrGone и
// битый архив пометку не стирают (критика П4, безопасность Д5).
func (s *Service) clearSkips(force map[string]bool, res *Update) {
	skip := prefs.LibraryNetOf(s.h.CfgDir()).Skip
	var ids []string
	for _, id := range res.done {
		if force[id] && slices.Contains(skip, id) {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return
	}
	prefs.UpdateLibraryNet(s.h.CfgDir(), func(n *prefs.LibraryNet) {
		for _, id := range ids {
			n.SetSkip(id, false)
		}
	})
}

// pickResult — итог «Скачать» решает Go: стоит ли нажатый набор после хода и, если нет, почему.
func (s *Service) pickResult(e fetch.Entry, have map[string]decor.PackInfo, st *decor.Store, res *Update) {
	_, stood := have[e.ID]
	res.Picked = slices.Contains(res.done, e.ID) || !stood && hasPack(st.Packs(), e.ID)
	if res.Picked {
		return
	}
	switch why := res.why[e.ID]; {
	case why != "":
		res.Why = why
	case stood:
		res.Why = "уже стоит"
	case e.NeedsApp(s.h.Version()):
		res.Why = whyApp(e)
	default:
		res.Why = "набор не поставлен"
	}
}

// installAll — наборы плана по одному. Не скачавшийся набор (404, битый архив, «архив не тот»,
// «библиотека заполнена», недопустимый id) не рвёт ход: он в res.Failed, дальше идут остальные.
// Ход обрывает то, что повторится у следующего: отмена, TLS, адрес, ошибка записи на диск, нет
// согласия, предел времени набора, ответ GitHub с ошибкой (429, 403, 407, 5xx — предел запросов
// или прокси) и вторая такая же неудача подряд — связь (обрыв, застой) или битый архив (иначе
// обрыв связи стоил бы застой по 30 с на каждый набор, а подмена тела — гигабайты впустую;
// безопасность Д3). Отмена во время установки: набор доустанавливается, следующий не начинается.
// origin — откуда наборы: net, а для примера (142) — demo.
func (s *Service) installAll(ctx context.Context, st *decor.Store, c *fetch.Client, todo []fetch.Entry, res *Update, prog progressFn, origin string) error {
	var last error // вид предыдущей неудачи (repeatKind), nil — успех или иная
	for _, e := range todo {
		if ctx.Err() != nil {
			return fetch.ErrCancelled
		}
		// Донор мог не встать (архив не тот, нечего ставить, поставлен из файла старее): план
		// считал его годным. Без донора шаблоны дополнения ссылаются на несуществующие картинки;
		// причина уже в итоге у самого донора.
		if e.Needs != "" && !hasPack(st.Packs(), e.Needs) {
			log.Printf("библиотека: дополнение %s пропущено — донор %s не поставлен", e.ID, e.Needs)
			res.why[e.ID] = "не поставлен набор, к которому это дополнение"
			continue
		}
		err := s.netInstall(ctx, st, c, e, res, prog, origin)
		if err == nil {
			last = nil
			continue
		}
		logNetErr("набор "+e.ID+" не поставлен", err)
		kind := repeatKind(err)
		var pf packFail
		if errors.As(err, &pf) && ctx.Err() != nil {
			// Отмена во время установки: ход — «отменён», а не отказ сети; причина набора остаётся.
			res.Failed = append(res.Failed, Skip{Name: e.Title, Why: failWhy(err)})
			res.why[e.ID] = failWhy(err)
			return fetch.ErrCancelled
		}
		if fatalNet(ctx, err) || kind != nil && kind == last {
			return err
		}
		last = kind
		res.Failed = append(res.Failed, Skip{Name: e.Title, Why: failWhy(err)})
		res.why[e.ID] = failWhy(err)
	}
	return nil
}

// errConn — вид неудачи «связь»: нет связи и застой — обрыв соединения CDN на одном архиве не
// рвёт ход, второй такой подряд рвёт.
var errConn = errors.New("связь")

// repeatKind — вид неудачи, вторая подряд которой рвёт ход; nil — другие считаются по одной.
func repeatKind(err error) error {
	switch {
	case errors.Is(err, fetch.ErrOffline), errors.Is(err, fetch.ErrTimeout):
		return errConn
	case errors.Is(err, fetch.ErrBroken):
		return fetch.ErrBroken
	}
	return nil
}

// fatalNet — ошибка, что повторится у любого следующего набора: ход обрывается. Ответ не 200
// (кроме 404 — нет одного ассета) — сервер ответит так же на следующий набор; ErrSlow — следующий
// набор упрётся так же (два предела подряд — до 40 мин впустую).
func fatalNet(ctx context.Context, err error) bool {
	var se *fetch.StatusError
	if errors.As(err, &se) && se.Code != http.StatusNotFound {
		return true
	}
	return ctx.Err() != nil || errors.Is(err, fetch.ErrCancelled) || errors.Is(err, fetch.ErrSlow) ||
		errors.Is(err, fetch.ErrTLS) || errors.Is(err, fetch.ErrAddress) || errors.Is(err, fetch.ErrDisk) ||
		errors.Is(err, ErrNoConsent)
}

// failWhy — причина в строке «не скачались»: у битого архива название уже в Name, текст ошибки
// повторил бы его.
func failWhy(err error) string {
	if errors.Is(err, fetch.ErrBroken) {
		return "скачался с ошибкой, повторите"
	}
	return err.Error()
}

// packFail — набор не встал по своей причине (архив не тот, нечего ставить): ход идёт дальше,
// причина — в res.Failed.
type packFail struct{ why string }

func (e packFail) Error() string { return e.why }

// netPlan — что ставить из pl.want: нет у нас или версия новее; удалённые пользователем (skip) —
// нет, если не в pl.force; новее стоящего не из сети (из файла) — пропуск с причиной. Причина
// «нужна новая программа» — только стоящим наборам: у нестоящего её покажет серая кнопка строки.
// Дополнение (needs) без донора — без причины и без тоста: донор в skip, не стоит и не годен к
// плану (иначе каждое «Обновить» качало бы его заново). Порядок строк каталога значения не
// имеет: донор идёт в todo раньше своих дополнений в любом месте списка. need — байты архивов.
func (s *Service) netPlan(st *decor.Store, cat fetch.Catalog, pl plan, res *Update) (todo []fetch.Entry, need int64) {
	have := haveOf(st)
	app := s.h.Version()
	skip := prefs.LibraryNetOf(s.h.CfgDir()).Skip
	type cand struct {
		e   fetch.Entry
		ok  bool   // годен к плану (до проверки донора)
		why string // причина пропуска; пусто при ok — молча
	}
	var cs []cand
	skipped := func(id string) bool { return slices.Contains(skip, id) && !pl.force[id] }
	demoID := s.h.Demo().ID
	for _, e := range cat.Packs {
		if !pl.want[e.ID] {
			continue
		}
		p, ok := have[e.ID]
		switch {
		case skipped(e.ID), e.Needs != "" && skipped(e.Needs):
			continue
		case e.ID == demoID && demoID != "":
			// Пример (origin demo) не «стоит из файла»: id зарезервирован, строка каталога с ним — ошибка издателя.
			cs = append(cs, cand{e: e, why: whyDemo})
		case ok && p.Origin == decor.OriginBuiltin:
			// До сверки версий: встроенный — версия 1, строка версии 1 иначе пропала бы молча.
			cs = append(cs, cand{e: e, why: whyBuiltin})
		case ok && p.Version >= e.Version:
			cs = append(cs, cand{e: e})
		case ok && p.Origin != decor.OriginNet:
			cs = append(cs, cand{e: e, why: whyFromFile})
		case e.NeedsApp(app):
			c := cand{e: e}
			if ok {
				c.why = whyApp(e)
			}
			cs = append(cs, c)
		default:
			cs = append(cs, cand{e: e, ok: true})
		}
	}
	// Донор: уже стоит или годен к плану. Проходы до неподвижной точки — цепочка дополнений
	// в любом порядке строк; цикл needs (a↔b) не берётся ни одним.
	planned := map[string]bool{}
	placed := make([]bool, len(cs))
	donorOK := func(e fetch.Entry) bool {
		_, stands := have[e.Needs]
		return e.Needs == "" || planned[e.Needs] || stands
	}
	for moved := true; moved; {
		moved = false
		for i, c := range cs {
			if placed[i] || !c.ok || !donorOK(c.e) {
				continue
			}
			placed[i], moved = true, true
			planned[c.e.ID] = true
			todo = append(todo, c.e)
			need += c.e.Size
		}
	}
	// Причины — в порядке каталога; дополнению без донора причина не нужна (молчим, как раньше).
	for _, c := range cs {
		if !c.ok && c.why != "" && donorOK(c.e) {
			res.Skipped = append(res.Skipped, Skip{Name: c.e.Title, Why: c.why})
		}
	}
	return todo, need
}

func hasPack(packs []decor.PackInfo, id string) bool {
	return slices.ContainsFunc(packs, func(p decor.PackInfo) bool { return p.ID == id })
}

// netInstall — один набор (origin — net, для примера demo): квота по размеру из каталога до скачивания, скачивание в
// Наборы/.dl-<id>.tmp, проверка архива и установка путём «из файла». Набор, который эта версия
// не ставит (манифест, нечего ставить, архив не тот), — packFail с причиной; сеть и диск — ошибка.
func (s *Service) netInstall(ctx context.Context, st *decor.Store, c *fetch.Client, e fetch.Entry, res *Update, prog progressFn, origin string) error {
	// Стоящая версия того же id при обновлении уходит: без вычета набор у предела не обновился
	// бы никогда. Точная сверка (общие ключи) — в InstallPack.
	old := st.PackDiskBytes(e.ID) // диск, а не PackInfo.Bytes (у набора из сети — размер zip)
	if st.Used()-old+e.Size > decor.MaxLibraryLen {
		return decor.ErrLibFull
	}
	tmp, err := st.DownloadPath(e.ID)
	if err != nil {
		return diskErr(e, err)
	}
	defer os.Remove(tmp) //nolint:errcheck // временный: нет файла — уже удалён (Windows: после закрытия архива)
	// Файл создаётся, пишется и читается одним дескриптором: по пути его заново не открывают
	// (FIFO или ссылка, подложенные на время сети, не повесят задание под мьютексом).
	f, err := fetch.OpenNew(tmp)
	if err != nil {
		return diskErr(e, err)
	}
	prog("download", e.Title, 0, e.Size)
	err = c.DownloadTo(ctx, e, f, func(done int64) { prog("download", e.Title, done, e.Size) })
	if err != nil {
		_ = f.Close()
		return err
	}
	res.Bytes += e.Size
	prog("install", e.Title, 0, 0)
	arc, err := pack.OpenFile(f, s.h.Version()) // закрывает f (и при ошибке)
	if err != nil {
		return packFail{err.Error()}
	}
	defer arc.Close() //nolint:errcheck // чтение архива: ошибка закрытия ничего не теряет
	// Архив подлинный (sha256 из подписанного каталога), но строка каталога — о другом наборе.
	if arc.Manifest.ID != e.ID || arc.Manifest.Version != e.Version || arc.SHA256 != e.SHA256 {
		return packFail{"архив не совпадает со списком наборов"}
	}
	skips := packSkips(arc.Manifest.Skipped)
	sg, err := st.StagePack(decorManifest(arc.Manifest), arc, origin, arc.SHA256)
	if sg != nil {
		defer sg.Close()
		if origin != decor.OriginFile { // в netInstall — net и demo (пример тоже из сети)
			sg.SetZipSize(e.Size) // подпись размера набора та же, что в «Можно скачать»
		}
		for _, k := range sg.Skipped {
			skips = append(skips, Skip(k))
		}
	}
	switch {
	case errors.Is(err, decor.ErrPackEmpty):
		return packFail{packEmptyErr(err, skips).Error()} // причины предметов — в тексте, а не отдельными строками
	case err != nil:
		return diskErr(e, err)
	}
	res.Skipped = append(res.Skipped, skips...)
	if sg.Ask == decor.AskSame || sg.Ask == decor.AskOlder {
		return nil // поставили «из файла» за время скачивания
	}
	info, err := st.InstallPack(sg, s.h.AlbumLibs())
	if err != nil {
		return diskErr(e, err)
	}
	res.titles = append(res.titles, info.Title)
	res.done = append(res.done, e.ID)
	if sg.Ask == decor.AskUpdate {
		res.Updated++
	} else {
		res.Added++
	}
	return nil
}

// diskErr — ошибка установки скачанного набора для тоста: квота и недопустимый id как есть (по
// набору, ход идёт дальше), предмет не сходится с манифестом (архив подлинный по sha каталога —
// ошибка издателя, не диска) — «скачался с ошибкой», остальное — постоянный текст без пути
// (fetch.ErrDisk, обрывает ход); в log.txt — только тип: текст ошибки диска может нести путь.
func diskErr(e fetch.Entry, err error) error {
	if errors.Is(err, decor.ErrLibFull) || errors.Is(err, decor.ErrPackEmpty) || errors.Is(err, decor.ErrBadPackID) {
		return err
	}
	if errors.Is(err, pack.ErrDamaged) {
		log.Printf("библиотека: набор %s — предмет не сходится с манифестом (%T)", e.ID, err)
		return fetch.Broken(e.Title)
	}
	log.Printf("библиотека: набор %s — ошибка установки (%T)", e.ID, err)
	return fetch.ErrDisk
}
