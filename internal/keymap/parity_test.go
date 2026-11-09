package keymap

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "перезаписать testdata/parity/keymap.json, дописать testdata/keymap-ids.txt")

var (
	parityPath  = filepath.Join("..", "..", "testdata", "parity", "keymap.json")
	historyPath = filepath.Join("..", "..", "testdata", "keymap-ids.txt")
)

// chordCase — вектор разбора сочетания: Go (Canon) и TS гоняют один список.
type chordCase struct {
	In  string `json:"in"`
	Out string `json:"out"`
}

// eventCase — вектор перевода события клавиатуры в сочетание (TS eventChord, design §3);
// Chord nil — не наше сочетание.
type eventCase struct {
	Key   string  `json:"key"`
	Code  string  `json:"code"`
	Ctrl  bool    `json:"ctrl,omitempty"`
	Shift bool    `json:"shift,omitempty"`
	Alt   bool    `json:"alt,omitempty"`
	Meta  bool    `json:"meta,omitempty"`
	Mac   bool    `json:"mac,omitempty"`
	Chord *string `json:"chord"`
}

type parity struct {
	View   View     `json:"view"`
	Codes  []string `json:"codes"`
	Chords struct {
		Valid   []chordCase `json:"valid"`
		Invalid []string    `json:"invalid"`
	} `json:"chords"`
	Events []eventCase `json:"events"`
}

var validChords = []chordCase{
	{"KeyZ", "KeyZ"}, {"Mod+KeyZ", "Mod+KeyZ"}, {"Shift+Mod+KeyZ", "Mod+Shift+KeyZ"},
	{"Shift+Alt+Mod+Digit1", "Mod+Alt+Shift+Digit1"}, {"Alt+Shift+F12", "Alt+Shift+F12"},
	{"Mod+Shift+BracketRight", "Mod+Shift+BracketRight"}, {"IntlBackslash", "IntlBackslash"},
	{"Shift+Slash", "Shift+Slash"}, {"Delete", "Delete"}, {"Mod+Alt+Shift+BracketRight", "Mod+Alt+Shift+BracketRight"},
}

var invalidChords = []string{
	"", "Mod", "Mod+", "+KeyZ", "Mod++KeyZ", "Mod+Mod+KeyZ", "Ctrl+KeyZ", "Cmd+KeyZ", "Meta+KeyZ",
	"keyz", "KeyZ+Mod", "Mod+KeyZ+KeyX", "Numpad1", "NumpadAdd", "F13", "AudioVolumeUp", " Mod+KeyZ",
	"Mod+KeyZ ", "mod+KeyZ", "Mod+Alt+Shift+Mod+KeyZ", strings.Repeat("Shift+", 5) + "KeyZ",
}

func ch(s string) *string { return &s }

var eventCases = []eventCase{
	{Key: "я", Code: "KeyZ", Ctrl: true, Chord: ch("Mod+KeyZ")},
	{Key: "z", Code: "KeyZ", Ctrl: true, Mac: true, Chord: nil}, // Mac: Ctrl не наш (074-20)
	{Key: "z", Code: "KeyZ", Meta: true, Mac: true, Chord: ch("Mod+KeyZ")},
	{Key: "z", Code: "KeyZ", Meta: true, Chord: nil}, // Windows: Win-клавиша не наша
	{Key: "Z", Code: "KeyZ", Ctrl: true, Shift: true, Chord: ch("Mod+Shift+KeyZ")},
	{Key: "я", Code: "", Chord: ch("KeyZ")}, // пустой code — по символу (074-26)
	{Key: "х", Code: "", Chord: ch("BracketLeft")},
	{Key: "ж", Code: "", Chord: ch("Semicolon")},
	{Key: "z", Code: "KeyW", Chord: ch("KeyZ")}, // AZERTY — по букве
	{Key: "&", Code: "Digit1", Chord: ch("Digit1")},
	{Key: "1", Code: "Numpad1", Chord: ch("Digit1")},
	{Key: "ArrowLeft", Code: "Numpad4", Chord: ch("ArrowLeft")}, // NumLock выключен
	{Key: "+", Code: "NumpadAdd", Ctrl: true, Chord: ch("Mod+Equal")},
	{Key: "-", Code: "NumpadSubtract", Ctrl: true, Chord: ch("Mod+Minus")},
	{Key: "Enter", Code: "NumpadEnter", Chord: ch("Enter")},
	{Key: ",", Code: "NumpadDecimal", Chord: ch("Period")},
	{Key: "*", Code: "NumpadMultiply", Chord: nil},
	{Key: "?", Code: "Slash", Shift: true, Chord: ch("Shift+Slash")},
	{Key: "?", Code: "Digit7", Shift: true, Chord: ch("Shift+Digit7")}, // «?» на русской раскладке
	{Key: "Ω", Code: "KeyZ", Alt: true, Mac: true, Chord: ch("Alt+KeyZ")},
	{Key: "Control", Code: "ControlLeft", Ctrl: true, Chord: nil},
	{Key: "Shift", Code: "ShiftLeft", Shift: true, Chord: nil},
	{Key: "AltGraph", Code: "AltRight", Ctrl: true, Alt: true, Chord: nil},
	{Key: "F5", Code: "F5", Chord: ch("F5")},
	{Key: "Delete", Code: "Delete", Chord: ch("Delete")},
	{Key: "AudioVolumeUp", Code: "AudioVolumeUp", Chord: nil},
	{Key: "\\", Code: "IntlBackslash", Chord: ch("IntlBackslash")},
	{Key: " ", Code: "Space", Shift: true, Chord: ch("Shift+Space")},
	{Key: "Unidentified", Code: "", Chord: nil},
}

func buildParity() parity {
	var p parity
	p.View = Build(nil)
	p.Codes = Codes()
	p.Chords.Valid = validChords
	p.Chords.Invalid = invalidChords
	p.Events = eventCases
	return p
}

// Файл, общий с тестами интерфейса: каталог, стандартный вид и векторы. Каталог поменялся —
// go test ./internal/keymap -run TestCatalogGolden -update и проверить дифф.
func TestCatalogGolden(t *testing.T) {
	data, err := json.MarshalIndent(buildParity(), "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, '\n')
	if *update {
		if err := os.WriteFile(parityPath, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	old, err := os.ReadFile(parityPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(bytes.ReplaceAll(old, []byte("\r\n"), []byte("\n")), data) {
		t.Fatalf("%s устарел: go test ./internal/keymap -run TestCatalogGolden -update", parityPath)
	}
	var back parity
	if err := json.Unmarshal(old, &back); err != nil || len(back.View.Commands) != len(catalog) {
		t.Fatalf("разбор: %v, команд %d", err, len(back.View.Commands))
	}
}

func TestCanonVectors(t *testing.T) {
	for _, c := range validChords {
		if got, ok := Canon(c.In); !ok || got != c.Out {
			t.Errorf("Canon(%q) = %q, %v; ждали %q", c.In, got, ok, c.Out)
		}
	}
	for _, s := range invalidChords {
		if got, ok := Canon(s); ok {
			t.Errorf("Canon(%q) = %q — ждали отказ", s, got)
		}
	}
	// Ожидаемое у векторов событий — само каноническое сочетание.
	for _, e := range eventCases {
		if e.Chord == nil {
			continue
		}
		if got, ok := Canon(*e.Chord); !ok || got != *e.Chord {
			t.Errorf("событие %+v: %q не каноническое", e, *e.Chord)
		}
	}
	for _, code := range Codes() {
		if strings.HasPrefix(code, "Numpad") {
			t.Errorf("цифровой блок в белом списке: %s", code)
		}
	}
}

func FuzzCanon(f *testing.F) {
	for _, c := range validChords {
		f.Add(c.In)
	}
	for _, s := range invalidChords {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		out, ok := Canon(s)
		if !ok {
			return
		}
		if len(out) > MaxChord {
			t.Fatalf("Canon(%q) = %q длиннее %d", s, out, MaxChord)
		}
		if again, ok2 := Canon(out); !ok2 || again != out {
			t.Fatalf("Canon не устойчив: %q → %q → %q, %v", s, out, again, ok2)
		}
		_ = ReservedReason(out)
	})
}

func readHistory(t *testing.T) []string {
	t.Helper()
	data, err := os.ReadFile(historyPath)
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, l := range strings.Split(string(data), "\n") {
		l = strings.TrimSpace(l)
		if l != "" && !strings.HasPrefix(l, "#") {
			ids = append(ids, l)
		}
	}
	return ids
}

// uncovered — id из истории выпусков, которых нет ни в каталоге, ни в Aliases, ни в Retired:
// своё сочетание пользователя у такой команды пропало бы молча.
func uncovered(history []string) []string {
	var out []string
	for _, id := range history {
		if _, ok := lookup(id); !ok && Aliases[id] == "" && !Retired[id] {
			out = append(out, id)
		}
	}
	return out
}

// История id (074-21): файл только дописывается; новая команда — -update допишет её id.
func TestRegistryHistoryCovered(t *testing.T) {
	hist := readHistory(t)
	var missing []string
	for _, x := range catalog {
		if !slices.Contains(hist, x.ID) {
			missing = append(missing, x.ID)
		}
	}
	if len(missing) > 0 && *update {
		f, err := os.OpenFile(historyPath, os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			t.Fatal(err)
		}
		_, err = f.WriteString(strings.Join(missing, "\n") + "\n")
		if cerr := f.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			t.Fatal(err)
		}
		hist, missing = readHistory(t), nil
	}
	if len(missing) > 0 {
		t.Errorf("новые id не в %s: %v (go test ./internal/keymap -run TestRegistryHistoryCovered -update)", historyPath, missing)
	}
	if u := uncovered(hist); len(u) > 0 {
		t.Errorf("id из выпусков пропали из каталога: %v — добавьте в Aliases (переименован) или Retired (убран)", u)
	}
	for old, n := range Aliases {
		if _, ok := lookup(n); !ok || old == n {
			t.Errorf("Aliases[%q] = %q: не команда каталога", old, n)
		}
		if _, ok := lookup(old); ok {
			t.Errorf("Aliases: %q — действующий id", old)
		}
	}
	for id := range Retired {
		if _, ok := lookup(id); ok {
			t.Errorf("Retired: %q — действующий id", id)
		}
	}
}

// Страж истории ловит пропавший id и молчит, когда тот переименован или убран.
func TestUncoveredGuard(t *testing.T) {
	defer restoreHistory(Aliases, Retired)
	if u := uncovered([]string{"undo", "goneCmd", "oldCmd"}); !slices.Equal(u, []string{"goneCmd", "oldCmd"}) {
		t.Errorf("uncovered = %v", u)
	}
	Aliases = map[string]string{"oldCmd": "undo"}
	Retired = map[string]bool{"goneCmd": true}
	if u := uncovered([]string{"undo", "goneCmd", "oldCmd"}); len(u) != 0 {
		t.Errorf("uncovered = %v, ждали пусто", u)
	}
}

func restoreHistory(a map[string]string, r map[string]bool) { Aliases, Retired = a, r }
