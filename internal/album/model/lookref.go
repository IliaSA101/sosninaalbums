package model

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Ссылка альбома на макет, предел номера шага и их зажим (131 design §2, §3): Load и каждая
// правка (FixLook). Сам макет (AlbumTpl, шаги, серия) и предел MaxLookSteps (Spread.LookStep —
// 1…MaxLookSteps) — look.go.

// LookRef — макет, по которому оформлен альбом (Project.Look, design §3): ключ файла макета
// my:<16 hex> (sha256 байтов), имя для тоста и окна (≤ MaxNameRunes) и где его искать после
// обновления набора — id набора и путь записи в манифесте (у «Моих» пусты). Путь в ФС не идёт:
// только поиск в реестре.
type LookRef struct {
	Key  string `json:"key"`
	Name string `json:"name,omitempty"`
	Pack string `json:"pack,omitempty"`
	Item string `json:"item,omitempty"`
}

// SameLook — a и b — один макет: тот же ключ либо та же непустая пара набор+запись (после
// обновления набора у макета новый ключ, а место в манифесте прежнее; design §3 «Поиск макета»).
// nil с любой стороны — не тот же.
func SameLook(a, b *LookRef) bool {
	if a == nil || b == nil {
		return false
	}
	return a.Key == b.Key || a.Pack != "" && a.Item != "" && a.Pack == b.Pack && a.Item == b.Item
}

// maxLookItem — предел пути записи в манифесте, байт (имя предмета набора — до 194).
const maxLookItem = 200

// ClampLookRef — ссылка из файла или от ядра к допустимой: ключ не по ValidLibKey — nil (макета
// нет); имя — CleanName; набор и запись — парой: id не по грамматике набора или путь с «..»,
// «\», «:», управляющими знаками, длиннее maxLookItem — оба пусты (макет ищется только по ключу).
func ClampLookRef(r *LookRef) *LookRef {
	if r == nil || !ValidLibKey(r.Key) {
		return nil
	}
	c := LookRef{Key: r.Key, Name: CleanName(r.Name)}
	if ValidPackID(r.Pack) && validLookItem(r.Item) {
		c.Pack, c.Item = r.Pack, r.Item
	}
	return &c
}

// ValidPackID — id набора ^[a-z0-9][a-z0-9-]{2,39}$, не устройство Windows: та же грамматика,
// что pack.ValidID (model его не импортирует; сверку держать тестом в app — остаток G2).
func ValidPackID(id string) bool {
	if len(id) < 3 || len(id) > 40 || id[0] == '-' {
		return false
	}
	for i := range len(id) {
		if c := id[i]; (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '-' {
			return false
		}
	}
	switch id {
	case "con", "prn", "aux", "nul":
		return false
	}
	return len(id) != 4 || !strings.HasPrefix(id, "com") && !strings.HasPrefix(id, "lpt") || id[3] < '0' || id[3] > '9'
}

// validLookItem — путь записи в манифесте: непустой, ≤ maxLookItem байт, UTF-8, без «..», «\»,
// «:», ведущего «/», управляющих и невидимых форматных знаков (Cf: bidi, нулевой ширины, BOM —
// путь в окне выглядел бы не тем, что ищется). Шире грамматики имён набора нарочно (0.28-131-11):
// новая версия наборов не должна стирать ссылку у прежней программы.
func validLookItem(s string) bool {
	if s == "" || len(s) > maxLookItem || !utf8.ValidString(s) || s[0] == '/' ||
		strings.Contains(s, "..") || strings.ContainsAny(s, `\:`) {
		return false
	}
	return !strings.ContainsFunc(s, func(r rune) bool { return unicode.IsControl(r) || unicode.Is(unicode.Cf, r) })
}

// FixLook — инварианты макета: ссылка зажата (ClampLookRef); без макета шагов нет; шаг вне
// 1…MaxLookSteps — 0; у обложки шага нет (её оформление — шаг 0 макета, Project.Look); лист без
// фона, мест, стикеров, имени и надписей шаг теряет — сверять со шагом нечего (design §3
// «Нормализация», 0.28-131-80). Зовут
// Load и конвейер правки до Normalize: лист с одним номером шага пуст (Spread.Empty).
func (p *Project) FixLook() {
	p.Look = ClampLookRef(p.Look)
	if p.Cover != nil && p.Cover.Sheet != nil {
		p.Cover.Sheet.LookStep = 0
	}
	for _, s := range p.Spreads {
		if p.Look == nil || s.LookStep < 0 || s.LookStep > MaxLookSteps || !s.lookDecor() {
			s.LookStep = 0
		}
	}
}

// lookDecor — у листа есть то, что ставит шаг макета: места, стикеры, свой фон, узор, фото фона,
// имя главы, надписи и заготовки. Имя и надписи держат номер, чтобы шаг без мест и фона (титул из
// одной надписи, стикер без файла на этом ПК) не терял его после постановки: иначе повтор макета
// создавал бы новый разворот (0.28-131-80).
func (s *Spread) lookDecor() bool {
	return s.Name != "" || len(s.Texts) > 0 || len(s.Grid) > 0 || len(s.Stickers) > 0 || s.BgL != "" || s.BgR != "" ||
		s.BgPhoto != nil || s.BgPhotoL != nil || s.BgPhotoR != nil || s.Pat != nil || s.PatL != nil || s.PatR != nil
}
