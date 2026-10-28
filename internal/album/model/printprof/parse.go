package printprof

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"reflect"
	"slices"
	"strings"
	"unicode/utf8"
)

var errFormat = errors.New("format: поддерживается только 1")

// Пределы разбора: размер проверяется до декодирования, счётчики — до разбора таблиц.
const (
	MaxSize     = 64 << 10 // байт файла профиля
	maxFormats  = 32
	maxProducts = 32
	maxVersion  = 999_999
	maxSpine    = 64  // строк таблицы корешка, как model.MaxSpineSteps
	maxSpreads  = 200 // как model.MaxSpreads
	maxBleed    = 10  // мм
	minPx       = 100 // размер файла разворота, px: как render.CheckSize сверху, снизу — не пятно
	maxPx       = 65535
	maxMegapx   = 400_000_000 // px[0]·px[1]
	// MaxName — название типографии, символов: оно в строках интерфейса и в имени файла «Поделиться».
	MaxName = 40
	// Custom — id «Своих размеров» обложки: профилю и изделиям его не занять.
	Custom = "custom"
)

// Parse разбирает профиль из JSON строго: лишние поля, мусор после значения, числа вне пределов,
// битые ссылки (default, формат в пределе) — ошибка. Числа приводятся к шагу 0,01 (корешок
// формулы B — 0,001), пустые списки — nil, поэтому Parse(Encode(p)) равен p.
func Parse(data []byte) (Profile, error) {
	var p Profile
	switch {
	case len(data) == 0:
		return p, errors.New("профиль: пустой файл")
	case len(data) > MaxSize:
		return p, fmt.Errorf("профиль: больше %d КБ", MaxSize>>10)
	case !utf8.Valid(data):
		return p, errors.New("профиль: не UTF-8")
	}
	if err := checkKeys(data, reflect.TypeFor[Profile]()); err != nil {
		return Profile{}, fmt.Errorf("профиль: %s", reason(err))
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&p); err != nil {
		return Profile{}, fmt.Errorf("профиль: %s", reason(err))
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return Profile{}, errors.New("профиль: лишние данные после значения")
	}
	p.normalize()
	if err := p.Validate(); err != nil {
		return Profile{}, fmt.Errorf("профиль %s: %w", q(p.ID), err)
	}
	return p, nil
}

// Encode — канонический JSON профиля (отступ 2 пробела, перевод строки в конце). Негодный
// профиль не кодируется.
func Encode(p Profile) ([]byte, error) {
	b, err := p.encode()
	if err != nil {
		return nil, fmt.Errorf("профиль %s: %w", q(p.ID), err)
	}
	return b, nil
}

// encode — проверка полей и JSON без HTML-экранирования: с ним «<» в тексте становится «\u003c»,
// и 15 КБ таких знаков раздуваются за MaxSize — файл «Поделиться» не открылся бы обратно.
// Размер записи — тоже условие годности: Parse(Encode(p)) == p держится для любого годного p.
func (p Profile) encode() ([]byte, error) {
	if err := p.check(); err != nil {
		return nil, err
	}
	b, err := marshal(p) // после check не падает; ошибку всё равно отдаём
	if err != nil {
		return nil, err
	}
	if len(b) > MaxSize {
		return nil, fmt.Errorf("в файле больше %d КБ — сократите тексты и таблицы", MaxSize>>10)
	}
	return b, nil
}

// marshal — JSON с отступом и без HTML-экранирования.
func marshal(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func r2(v float64) float64 { return math.Round(v*100) / 100 }

func r2p(p *Pair) *Pair {
	if p == nil {
		return nil
	}
	return &Pair{r2(p[0]), r2(p[1])}
}

func r2s(v []float64) []float64 {
	if len(v) == 0 {
		return nil
	}
	out := make([]float64, len(v))
	for i, x := range v {
		out[i] = r2(x)
	}
	return out
}

// normalize приводит числа к шагу и пустые списки к nil. NaN и Inf из JSON не приходят, но
// профиль бывает собран в коде — Validate их отвергнет.
func (p *Profile) normalize() {
	p.Page.Safe, p.Page.Bleed = r2(p.Page.Safe), r2(p.Page.Bleed)
	if len(p.Formats) == 0 {
		p.Formats = nil
	}
	for i := range p.Formats {
		f := &p.Formats[i]
		f.Page = Pair{r2(f.Page[0]), r2(f.Page[1])}
		f.Safe, f.Bleed, f.Wrap = r2(f.Safe), r2(f.Bleed), r2(f.Wrap)
		if f.Side != nil {
			s := Pair{r2(f.Side[0]), r2(f.Side[1])}
			f.Side = &s
		}
		if g := f.Guide; g != nil {
			g.Bleed, g.Fold = r2p(g.Bleed), r2p(g.Fold)
		}
	}
	if c := p.Cover; c != nil {
		c.Wrap, c.Safe, c.SpineSafe = r2(c.Wrap), r2(c.Safe), r2(c.SpineSafe)
		c.FoldSafe, c.GuideSpineSafe = r2(c.FoldSafe), r2(c.GuideSpineSafe)
	}
	if len(p.Products) == 0 {
		p.Products = nil
	}
	for i := range p.Products {
		pr := &p.Products[i]
		pr.Spine = r2s(pr.Spine)
		if len(pr.SpineBy) == 0 {
			pr.SpineBy = nil
		}
		for k, t := range pr.SpineBy {
			pr.SpineBy[k] = r2s(t)
		}
		pr.Wrap, pr.Hinge, pr.A = r2(pr.Wrap), r2(pr.Hinge), r2(pr.A)
		pr.B = math.Round(pr.B*1000) / 1000 // как ClampCoverSpec
		if len(pr.Limits) == 0 {
			pr.Limits = nil
		}
	}
}

// Validate проверяет профиль: пределы чисел и размеров списков, имена, ссылки между полями,
// сайты адресов и размер записи (Encode).
func (p Profile) Validate() error {
	_, err := p.encode()
	return err
}

// check — Validate без размера записи.
func (p Profile) check() error {
	if p.Format != FormatVersion {
		return errFormat
	}
	switch {
	case !ValidID(p.ID) || len(p.ID) > MaxProfileID || p.ID == Custom:
		return fmt.Errorf("id: нужны латиница, цифры и «-», 2–%d знаков", MaxProfileID)
	case p.Version < 1 || p.Version > maxVersion:
		return fmt.Errorf("version: от 1 до %d", maxVersion)
	}
	for _, c := range []struct {
		what, s string
	}{{"name", p.Name}, {"gen", p.Gen}} {
		if err := checkLabel(c.what, c.s); err != nil {
			return err
		}
	}
	if utf8.RuneCountInString(p.Name) > MaxName {
		return fmt.Errorf("name: длиннее %d символов", MaxName)
	}
	if err := p.checkPage(); err != nil {
		return err
	}
	if err := p.checkProducts(); err != nil {
		return err
	}
	if p.Default != "" && !slices.ContainsFunc(p.Products, func(pr Product) bool { return pr.ID == p.Default }) {
		return errors.New("default: такого изделия нет")
	}
	if p.Default == "" && p.Products != nil {
		return errors.New("default: нужно изделие по умолчанию")
	}
	if p.Export != nil {
		if err := p.Export.validate(); err != nil {
			return err
		}
	}
	return p.checkHosts()
}

func inRange(v, lo, hi float64) bool { return !math.IsNaN(v) && v >= lo && v <= hi }

func (p Profile) checkPage() error {
	g := p.Page
	switch {
	case g.DPI < 72 || g.DPI > 1200:
		return errors.New("page.dpi: от 72 до 1200")
	case !inRange(g.Safe, 0, 30) || !inRange(g.Bleed, 0, maxBleed):
		return fmt.Errorf("page: зона от 0 до 30 мм, обрез до %d мм", maxBleed)
	}
	if err := checkTextMax("page.hint", g.Hint, false, maxHint); err != nil {
		return err
	}
	if err := checkTemplate("page.hint", g.Hint, "{safe}"); err != nil {
		return err
	}
	if len(p.Formats) > maxFormats {
		return fmt.Errorf("formats: больше %d", maxFormats)
	}
	seen := map[string]bool{}
	for _, f := range p.Formats {
		if err := checkLabel("formats.name", f.Name); err != nil {
			return err
		}
		if seen[f.Name] {
			return fmt.Errorf("formats: подпись %s повторена", q(f.Name))
		}
		seen[f.Name] = true
		if !inRange(f.Page[0], 50, 1000) || !inRange(f.Page[1], 50, 1000) {
			return fmt.Errorf("формат %s: страница от 50 до 1000 мм", q(f.Name))
		}
		if !inRange(f.Safe, 0, 30) || !inRange(f.Bleed, 0, maxBleed) || !inRange(f.Wrap, 0, 50) {
			return fmt.Errorf("формат %s: зона, обрез и заворот вне пределов", q(f.Name))
		}
		if s := f.Side; s != nil && (!inRange(s[0], 50, 1000) || !inRange(s[1], 50, 1000)) {
			return fmt.Errorf("формат %s: сторонка от 50 до 1000 мм", q(f.Name))
		}
		if err := f.checkGuide(); err != nil {
			return err
		}
		if px := f.Px; px != nil && (px[0] < minPx || px[1] < minPx || px[0] > maxPx || px[1] > maxPx || px[0]*px[1] > maxMegapx) {
			return fmt.Errorf("формат %s: px от %d до %d и не больше 400 Мп", q(f.Name), minPx, maxPx)
		}
	}
	return nil
}

// checkGuide: направляющие не пусты; обрез — как у формата, загиб от края листа до 50 мм и только у
// формата с сторонкой (без обложки по шаблону линии загиба рисовать не на чем).
func (f PageFormat) checkGuide() error {
	g := f.Guide
	if g == nil {
		return nil
	}
	if g.Bleed == nil && g.Fold == nil {
		return fmt.Errorf("формат %s: guide пуст", q(f.Name))
	}
	if b := g.Bleed; b != nil && (!inRange(b[0], 0, maxBleed) || !inRange(b[1], 0, maxBleed)) {
		return fmt.Errorf("формат %s: guide.bleed от 0 до %d мм", q(f.Name), maxBleed)
	}
	if d := g.Fold; d != nil {
		if f.Side == nil {
			return fmt.Errorf("формат %s: guide.fold только у формата с side", q(f.Name))
		}
		if !inRange(d[0], 0.01, 50) || !inRange(d[1], 0.01, 50) {
			return fmt.Errorf("формат %s: guide.fold от 0,01 до 50 мм", q(f.Name))
		}
	}
	return nil
}

func (p Profile) checkProducts() error {
	if c := p.Cover; c != nil && (!inRange(c.Wrap, 0, 50) || !inRange(c.Safe, 0, 30) || !inRange(c.SpineSafe, 0, 30) ||
		!inRange(c.FoldSafe, 0, 30) || !inRange(c.GuideSpineSafe, 0, 30)) {
		return errors.New("cover: заворот до 50 мм, линии до 30")
	}
	if len(p.Products) > maxProducts {
		return fmt.Errorf("products: больше %d", maxProducts)
	}
	ids := map[string]bool{}
	for _, pr := range p.Products {
		if !ValidID(pr.ID) || ids[pr.ID] || !strings.HasPrefix(pr.ID, p.ID+"-") {
			return fmt.Errorf("изделие %s: id негодный или повторён", q(pr.ID))
		}
		ids[pr.ID] = true
		if err := p.checkProduct(pr); err != nil {
			return fmt.Errorf("изделие %s: %w", q(pr.ID), err)
		}
	}
	return nil
}

func (p Profile) checkProduct(pr Product) error {
	if err := checkLabel("name", pr.Name); err != nil {
		return err
	}
	for _, c := range []struct{ what, s string }{{"pages", pr.Pages}, {"item", pr.Item}} {
		if err := checkText(c.what, c.s, false); err != nil {
			return err
		}
	}
	if err := checkURL("url", pr.URL); err != nil {
		return err
	}
	if !inRange(pr.Wrap, 0, 50) || !inRange(pr.Hinge, 0, 20) || !inRange(pr.A, 0, 100) || !inRange(pr.B, 0, 5) {
		return errors.New("заворот, расстав, A, B вне пределов")
	}
	if err := checkSpine("spine", pr.Spine); err != nil {
		return err
	}
	if len(pr.Spine) > 0 && (pr.A != 0 || pr.B != 0) {
		return errors.New("нужна таблица корешка или формула A + B·n, не обе")
	}
	for name, t := range pr.SpineBy {
		if _, ok := p.FormatByName(name); !ok {
			return fmt.Errorf("spineBy: формата %s нет", q(name))
		}
		if len(t) == 0 {
			return fmt.Errorf("spineBy %s: пусто", q(name))
		}
		if err := checkSpine("spineBy "+q(name), t); err != nil {
			return err
		}
	}
	return p.checkLimits(pr)
}

// checkLimits: число строк отдельно не ограничено — формат в строке уникален, форматов ≤ maxFormats,
// поэтому строк не больше maxFormats+1 (общая без формата); design §6 писал «≤ 128» (решение 0.26-123-44).
func (p Profile) checkLimits(pr Product) error {
	seen := map[string]bool{}
	for _, l := range pr.Limits {
		if l.Format != "" {
			if _, ok := p.FormatByName(l.Format); !ok {
				return fmt.Errorf("limits: формата %s нет", q(l.Format))
			}
		}
		if seen[l.Format] {
			return fmt.Errorf("limits: строка формата %s повторена", q(l.Format))
		}
		seen[l.Format] = true
		top := l.Max // max 0 — верх таблицы корешка формата (model.addProducts берёт его же)
		if l.Max == 0 {
			t := pr.SpineFor(l.Format)
			if len(t) == 0 {
				return fmt.Errorf("limits %s: max 0 берёт верх таблицы корешка, а таблицы нет", q(l.Format))
			}
			top = int(t[len(t)-2])
		}
		switch {
		case l.Min < 0 || l.Min > maxSpreads || l.Max < 0 || l.Max > maxSpreads:
			return fmt.Errorf("limits %s: от 0 до %d", q(l.Format), maxSpreads)
		case l.Min > top:
			return fmt.Errorf("limits %s: min больше max (верха таблицы корешка)", q(l.Format))
		}
	}
	return nil
}

// checkSpine — плоская таблица «до N, мм»: пары, N по возрастанию от 1 до maxSpreads, мм до 100.
func checkSpine(what string, t []float64) error {
	if len(t)%2 != 0 || len(t)/2 > maxSpine {
		return fmt.Errorf("%s: пар от 1 до %d", what, maxSpine)
	}
	prev := 0.0
	for i := 0; i < len(t); i += 2 {
		n, mm := t[i], t[i+1]
		if n != math.Trunc(n) || n <= prev || n > maxSpreads || !inRange(mm, 0, 100) {
			return fmt.Errorf("%s: строка %d вне пределов или не по возрастанию", what, i/2+1)
		}
		prev = n
	}
	return nil
}

func (e Export) validate() error {
	for _, c := range []struct {
		what, s string
		req     bool
	}{
		{"export.acc", e.Acc, true}, {"export.title", e.Title, true}, {"export.coverWord", e.CoverWord, true},
		{"export.pagesCap", e.PagesCap, false}, {"export.plain", e.Plain, true}, {"export.withCover", e.WithCover, false},
		{"export.done", e.Done, true}, {"export.site", e.Site, true}, {"export.siteNoItem", e.SiteNoItem, false},
		{"export.siteButton", e.SiteButton, true},
	} {
		if err := checkText(c.what, c.s, c.req); err != nil {
			return err
		}
	}
	if err := checkTemplate("export.site", e.Site, "{item}"); err != nil {
		return err
	}
	if !prefixRe.MatchString(e.Prefix) {
		return errors.New("export.prefix: до 8 знаков, латиница, цифры, «-» и «_»")
	}
	if e.Quality != 0 && e.Quality != 95 && e.Quality != 100 {
		return errors.New("export.quality: 0, 95 или 100")
	}
	return checkURL("export.url", e.URL)
}
