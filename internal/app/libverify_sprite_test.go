package app

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Лист образцов набора (спека 144 Г4, security Н5 и Н7): library-verify -sprite <zip> -out <папка> —
// клетки из байтов архива, baseline JPEG, preview.json со строками и items = sha256[:16] записей манифеста.

// spMeta — preview.json листа.
type spMeta struct {
	W    int `json:"w"`
	H    int `json:"h"`
	Rows []struct {
		Kind  string   `json:"kind"`
		W     int      `json:"w"`
		H     int      `json:"h"`
		Items []string `json:"items"`
	} `json:"rows"`
}

// spSum — sha256[:16] байтов предмета: так items листа называют запись манифеста.
func spSum(data []byte) string {
	s := sha256.Sum256(data)
	return hex.EncodeToString(s[:])[:16]
}

// spRead — лист и preview.json набора id v1 из папки out.
func spRead(t *testing.T, out, id string) ([]byte, spMeta) {
	t.Helper()
	jpg, err := os.ReadFile(filepath.Join(out, id+"-1-preview.jpg"))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(out, id+"-1-preview.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m spMeta
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	return jpg, m
}

// spNoFiles — в папке out нет ни листа, ни временных файлов.
func spNoFiles(t *testing.T, out string) {
	t.Helper()
	if m, _ := filepath.Glob(filepath.Join(out, "*")); len(m) != 0 {
		t.Errorf("при провале в -out остались файлы: %v", m)
	}
}

// spSet — набор макетов minApp 0.28.0 (запуск с -app 0.28.0): фоны, стикеры, рамка, шаблоны, макет альбома.
func spSet(t *testing.T, id string, files ...pfile) string {
	t.Helper()
	return patchManifest(t, packZip(t, id, 1, files...), map[string]any{"minApp": "0.28.0"})
}

// лист из zip: все виды, baseline, items = sha256[:16], клетки — байты архива
func (e spEnv) sheet(t *testing.T) {
	out := testkit.TempDir(t)
	text, code := e.run(t, "-sprite", spSet(t, "sprset", e.all...), "-out", out)
	if code != 0 || !strings.Contains(text, "лист: sprset-1-preview.jpg") || !strings.HasSuffix(strings.TrimSpace(text), "OK") {
		t.Fatalf("код %d\n%s", code, text)
	}
	jpg, meta := spRead(t, out, "sprset")
	cfg, err := jpeg.DecodeConfig(bytes.NewReader(jpg))
	if err != nil || cfg.Width != meta.W || cfg.Height != meta.H || cfg.ColorModel != color.YCbCrModel {
		t.Fatalf("кадр %+v (%v), preview.json %d×%d", cfg, err, meta.W, meta.H)
	}
	if i := bytes.Index(jpg, []byte{0xff, 0xc0}); i < 0 || bytes.Contains(jpg, []byte{0xff, 0xc2}) { // SOF0, не прогрессивный
		t.Errorf("не baseline: SOF0 на %d", i)
	}
	if len(jpg) > 384<<10 {
		t.Errorf("лист %d байт больше предела 384 КиБ", len(jpg))
	}
	want := map[string][]string{
		"bg": {spSum(e.files[0].data)}, "sticker": {spSum(e.files[1].data)}, "frame": {spSum(e.files[2].data)},
		"spread": {spSum(e.files[3].data), spSum(e.files[4].data)}, "album": {spSum(e.look.data)},
	}
	var kinds []string
	for _, r := range meta.Rows {
		kinds = append(kinds, r.Kind)
		if strings.Join(r.Items, ",") != strings.Join(want[r.Kind], ",") {
			t.Errorf("%s: items %v, нужно %v", r.Kind, r.Items, want[r.Kind])
		}
		if r.W < 32 || r.W > 480 || r.H < 32 || r.H > 480 || len(r.Items)*r.W > meta.W {
			t.Errorf("%s: клетка %d×%d, лист %d×%d", r.Kind, r.W, r.H, meta.W, meta.H)
		}
	}
	if strings.Join(kinds, ",") != "bg,sticker,frame,spread,album" {
		t.Errorf("ряды %v", kinds)
	}
	// Клетка фона — пиксели фона из архива (тон 0xe0, 0xc8, 0xd8), а не пустое место.
	img, err := jpeg.Decode(bytes.NewReader(jpg))
	if err != nil {
		t.Fatal(err)
	}
	r, gg, b, _ := img.At(meta.Rows[0].W/2, meta.Rows[0].H/2).RGBA()
	if d := func(a uint32, w int) int { return int(a>>8) - w }; abs(d(r, 0xe0)) > 12 || abs(d(gg, 0xc8)) > 12 || abs(d(b, 0xd8)) > 12 {
		t.Errorf("клетка фона не того цвета: %d %d %d", r>>8, gg>>8, b>>8)
	}
	// Круг: повторный прогон того же zip — тот же лист байт в байт.
	out2 := testkit.TempDir(t)
	if text, code := e.run(t, "-sprite", spSet(t, "sprset", e.all...), "-out", out2); code != 0 {
		t.Fatalf("повтор: код %d\n%s", code, text)
	}
	jpg2, _ := spRead(t, out2, "sprset")
	j1, _ := os.ReadFile(filepath.Join(out, "sprset-1-preview.json"))
	j2, _ := os.ReadFile(filepath.Join(out2, "sprset-1-preview.json"))
	if !bytes.Equal(jpg, jpg2) || !bytes.Equal(j1, j2) {
		t.Errorf("повторный -sprite дал другой лист: %d и %d байт", len(jpg), len(jpg2))
	}
}

// отбор по умолчанию — первые по манифесту: 8 фонов, 12 стикеров
func (e spEnv) caps(t *testing.T) {
	var fs []pfile
	var bgs, sts []string
	for i := range 10 {
		d := lvBg(t, 800, 600, uint8(20*i))
		fs = append(fs, pfile{name: "bg/b" + itoa(i) + ".jpg", kind: "bg", data: d})
		bgs = append(bgs, spSum(d))
	}
	for i := range 14 {
		d := lvSticker(t, uint8(10*i))
		fs = append(fs, pfile{name: "stickers/s" + itoa(i) + ".png", kind: "sticker", data: d})
		sts = append(sts, spSum(d))
	}
	out := testkit.TempDir(t)
	text, code := e.run(t, "-sprite", spSet(t, "many", fs...), "-out", out)
	if code != 0 {
		t.Fatalf("код %d\n%s", code, text)
	}
	_, meta := spRead(t, out, "many")
	if len(meta.Rows) != 2 || strings.Join(meta.Rows[0].Items, ",") != strings.Join(bgs[:8], ",") || strings.Join(meta.Rows[1].Items, ",") != strings.Join(sts[:12], ",") {
		t.Errorf("отбор: %+v", meta.Rows)
	}
	if meta.W != 12*160 || meta.H != 160+160 { // ширина — самый длинный ряд, высота — сумма рядов
		t.Errorf("лист %d×%d", meta.W, meta.H)
	}
}

// -pick: свои записи по именам манифеста; неизвестное поле, имя, вид, повтор, >12 — отказ без файлов
func (e spEnv) pick(t *testing.T) {
	var fs []pfile
	for i := range 4 {
		fs = append(fs, pfile{name: "bg/b" + itoa(i) + ".jpg", kind: "bg", data: lvBg(t, 800, 600, uint8(30*i))})
	}
	fs = append(fs, pfile{name: "stickers/dot.png", kind: "sticker", data: lvSticker(t, 0x50)})
	zipFile := spSet(t, "pickset", fs...)
	pick := func(body string) string {
		p := filepath.Join(testkit.TempDir(t), "pick.json")
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	out := testkit.TempDir(t)
	if text, code := e.run(t, "-sprite", zipFile, "-out", out, "-pick", pick(`{"bg": ["bg/b3.jpg", "bg/b1.jpg"]}`)); code != 0 {
		t.Fatalf("код %d\n%s", code, text)
	}
	_, meta := spRead(t, out, "pickset")
	if got, want := strings.Join(meta.Rows[0].Items, ","), spSum(fs[3].data)+","+spSum(fs[1].data); got != want || meta.Rows[1].Kind != "sticker" {
		t.Errorf("свой отбор: %+v", meta.Rows)
	}
	var wide []pfile // 13 разных фонов: больше клеток ряда (12), хотя все имена есть в манифесте
	var names []string
	for i := range 13 {
		wide = append(wide, pfile{name: "bg/w" + itoa(i) + ".jpg", kind: "bg", data: lvBg(t, 800, 600, uint8(15*i))})
		names = append(names, `"bg/w`+itoa(i)+`.jpg"`)
	}
	if text, code := e.run(t, "-sprite", spSet(t, "wideset", wide...), "-out", testkit.TempDir(t), "-pick", pick(`{"bg": [`+strings.Join(names, ",")+`]}`)); code != 1 || !strings.Contains(text, "не больше 12") {
		t.Errorf("13 клеток в ряду: код %d\n%s", code, text)
	}
	if _, code := e.run(t, "-sprite", spSet(t, "wideset", wide...), "-out", testkit.TempDir(t), "-pick", pick(`{"bg": [`+strings.Join(names[:12], ",")+`]}`)); code != 0 {
		t.Errorf("12 клеток в ряду: код %d", code)
	}
	for name, body := range map[string]string{
		"неизвестное поле": `{"bgs": ["bg/b0.jpg"]}`, "нет записи": `{"bg": ["bg/nope.jpg"]}`, "не тот вид": `{"bg": ["stickers/dot.png"]}`,
		"повтор": `{"bg": ["bg/b0.jpg", "bg/b0.jpg"]}`, "не объект": `[]`, "пусто": ``, "лишнее после": `{} {}`,
		"огромный": `{"bg": ["` + strings.Repeat("x", 70<<10) + `"]}`,
	} {
		o := testkit.TempDir(t)
		text, code := e.run(t, "-sprite", zipFile, "-out", o, "-pick", pick(body))
		if code != 1 || strings.Contains(text, "panic") {
			t.Errorf("%s: код %d\n%s", name, code, text)
		}
		spNoFiles(t, o)
	}
	if _, code := e.run(t, "-pick", pick(`{}`), zipFile); code != 2 {
		t.Errorf("-pick без -sprite: код %d", code)
	}
}

// плохие входы — ошибка вместо паники, в -out ничего
func (e spEnv) badInputs(t *testing.T) {
	good := spSet(t, "okset", e.all...)
	raw, err := os.ReadFile(good)
	if err != nil {
		t.Fatal(err)
	}
	dir := testkit.TempDir(t)
	write := func(name string, data []byte) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, data, 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	huge := write("huge.zip", nil)
	if err := os.Truncate(huge, 301<<20); err != nil {
		t.Fatal(err)
	}
	for name, p := range map[string]string{
		"пустой": write("empty.zip", nil), "усечённый": write("cut.zip", raw[:len(raw)/2]), "не zip": write("junk.zip", bytes.Repeat([]byte("не zip "), 400)),
		"огромный": huge, "папка": dir, "нет файла": filepath.Join(dir, "nope.zip"), "22 байта": write("tail.zip", raw[:22]),
	} {
		out := testkit.TempDir(t)
		text, code := e.run(t, "-sprite", p, "-out", out)
		if code != 1 || strings.Contains(text, "panic") || strings.Contains(text, "goroutine") || !strings.Contains(text, "ПРОВАЛ") {
			t.Errorf("%s: код %d\n%s", name, code, text)
		}
		spNoFiles(t, out)
	}
	out := testkit.TempDir(t)
	for name, args := range map[string][]string{
		"без -out": {"-sprite", good}, "с набором в аргументах": {"-sprite", good, "-out", out, good},
		"относительный zip": {"-sprite", "rel.zip", "-out", out}, "относительный -out": {"-sprite", good, "-out", "rel"},
		"относительный -pick": {"-sprite", good, "-out", out, "-pick", "rel.json"},
	} {
		if text, code := e.run(t, args...); code != 2 {
			t.Errorf("%s: код %d\n%s", name, code, text)
		}
	}
	spNoFiles(t, out)
}

// провал донора (-with) и сбой записи пары: лист не пишется, а половины пары и .tmp не остаётся
func (e spEnv) pairAndDonor(t *testing.T) {
	good := spSet(t, "okset", e.all...)
	junk := filepath.Join(testkit.TempDir(t), "donor.zip")
	if err := os.WriteFile(junk, bytes.Repeat([]byte("не zip "), 300), 0o644); err != nil {
		t.Fatal(err)
	}
	out := testkit.TempDir(t)
	if text, code := e.run(t, "-with", junk, "-sprite", good, "-out", out); code != 1 || !strings.Contains(text, "ПРОВАЛ") {
		t.Errorf("битый донор: код %d\n%s", code, text)
	}
	spNoFiles(t, out)
	if text, code := e.run(t, "-pick", "x.json", "-out", out); code != 2 {
		t.Errorf("-pick без -sprite: код %d\n%s", code, text)
	}
	// на месте preview.json — непустая папка: второй Rename падает, уже поставленный .jpg убирается
	out = testkit.TempDir(t)
	blocker := filepath.Join(out, "okset-1-preview.json")
	if err := os.MkdirAll(filepath.Join(blocker, "x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if text, code := e.run(t, "-sprite", good, "-out", out); code != 1 || !strings.Contains(text, "ПРОВАЛ") {
		t.Errorf("сбой записи пары: код %d\n%s", code, text)
	}
	if m, _ := filepath.Glob(filepath.Join(out, "*")); len(m) != 1 || m[0] != blocker {
		t.Errorf("после сбоя записи в -out лишнее: %v", m)
	}
}

// шумный лист не выходит за 384 КиБ: качество снижается
func (e spEnv) noisy(t *testing.T) {
	noise := func(w, h int) []byte { // шум в размер клетки: ни сжатие JPEG, ни масштаб его не прячут
		img := image.NewRGBA(image.Rect(0, 0, w, h))
		if _, err := rand.Read(img.Pix); err != nil {
			t.Fatal(err)
		}
		for j := 3; j < len(img.Pix); j += 4 {
			img.Pix[j] = 255
		}
		return lvPNG(t, img)
	}
	var fs []pfile
	var names []string
	for i := range 12 {
		names = append(names, `"bg/n`+itoa(i)+`.png"`)
		fs = append(fs, pfile{name: "bg/n" + itoa(i) + ".png", kind: "bg", data: noise(240, 160)})
	}
	for i := range 12 {
		fs = append(fs, pfile{name: "stickers/n" + itoa(i) + ".png", kind: "sticker", data: noise(160, 160)})
	}
	pick := filepath.Join(testkit.TempDir(t), "pick.json") // 12 фонов вместо 8 по умолчанию
	if err := os.WriteFile(pick, []byte(`{"bg": [`+strings.Join(names, ",")+`]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	out := testkit.TempDir(t)
	text, code := e.run(t, "-sprite", spSet(t, "noisy", fs...), "-out", out, "-pick", pick)
	jpg, _ := spRead(t, out, "noisy")
	if code != 0 || len(jpg) > 384<<10 || strings.Contains(text, "(q80)") || !strings.Contains(text, "(q") {
		t.Fatalf("код %d, лист %d байт: качество должно было упасть ниже 80\n%s", code, len(jpg), text)
	}
}

// сквозной: -sprite → library-pack --preview-only; архив и его sha256 не меняются
func (e spEnv) e2e(t *testing.T) {
	nodeBin, err := exec.LookPath("node")
	if err != nil {
		t.Skip("нет node в PATH")
	}
	zipFile := spSet(t, "e2e-set", e.all...)
	zipBefore, err := os.ReadFile(zipFile)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(zipBefore)
	out := testkit.TempDir(t)
	if text, code := e.run(t, "-sprite", zipFile, "-out", out); code != 0 {
		t.Fatalf("код %d\n%s", code, text)
	}
	// Каталог со строкой набора (sha256 — опубликованного zip) и ключ подписи — только для этого теста.
	work := testkit.TempDir(t)
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	keyFile := filepath.Join(work, "key.pem")
	if err := os.WriteFile(keyFile, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), 0o600); err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(map[string]any{"format": 1, "seq": 7, "packs": []map[string]any{
		{"id": "e2e-set", "version": 1, "minApp": "0.28.0", "title": "Набор e2e-set", "sha256": hex.EncodeToString(sum[:]), "size": len(zipBefore)}}})
	if err != nil {
		t.Fatal(err)
	}
	env, _ := json.Marshal(map[string]string{"sig": base64.StdEncoding.EncodeToString(ed25519.Sign(priv, body)), "body": base64.StdEncoding.EncodeToString(body)})
	catFile := filepath.Join(work, "catalog.json")
	if err := os.WriteFile(catFile, env, 0o644); err != nil {
		t.Fatal(err)
	}
	script, err := filepath.Abs(filepath.Join("..", "..", "scripts", "library-pack.mjs"))
	if err != nil {
		t.Fatal(err)
	}
	b, err := exec.Command(nodeBin, script, "--preview-only", "e2e-set="+zipFile, "--sheets", out, "--catalog", catFile, "--key", keyFile).CombinedOutput()
	if err != nil {
		t.Fatalf("library-pack --preview-only: %v\n%s", err, b)
	}
	var got struct {
		Body string `json:"body"`
	}
	raw, _ := os.ReadFile(catFile)
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	inner, _ := base64.StdEncoding.DecodeString(got.Body)
	var cat struct {
		Seq   int `json:"seq"`
		Packs []struct {
			SHA256  string `json:"sha256"`
			Preview struct {
				SHA256 string `json:"sha256"`
				Size   int    `json:"size"`
				W, H   int
				Rows   []json.RawMessage `json:"rows"`
			} `json:"preview"`
		} `json:"packs"`
	}
	if err := json.Unmarshal(inner, &cat); err != nil {
		t.Fatal(err)
	}
	jpg, meta := spRead(t, out, "e2e-set")
	ps := sha256.Sum256(jpg)
	p := cat.Packs[0]
	if cat.Seq != 8 || p.SHA256 != hex.EncodeToString(sum[:]) || p.Preview.SHA256 != hex.EncodeToString(ps[:]) || p.Preview.Size != len(jpg) || p.Preview.W != meta.W || p.Preview.H != meta.H || len(p.Preview.Rows) != len(meta.Rows) {
		t.Errorf("каталог после --preview-only: %+v, лист %d байт %d×%d", cat, len(jpg), meta.W, meta.H)
	}
	if after, _ := os.ReadFile(zipFile); !bytes.Equal(after, zipBefore) {
		t.Error("архив набора изменился")
	}
}

// spEnv — собранный скрипт и годный набор: фон, стикер, рамка, два шаблона и макет альбома.
type spEnv struct {
	g     lvTool
	files []pfile
	look  pfile
	all   []pfile
}

// run — запуск с версией 0.28.0 (wails.json дерева может быть старше).
func (e spEnv) run(t *testing.T, args ...string) (string, int) {
	t.Helper()
	return e.g.run(t, append([]string{"-app", "0.28.0"}, args...)...)
}

func TestLibraryVerifySprite(t *testing.T) {
	files, bg, _ := lvGood(t)
	// Макет альбома: шаг-разворот и обложка с лицом — клетка берёт лицо обложки.
	front := model.SpreadTpl{V: model.TplV, Pages: 1, Ratio: 0.67, Places: []model.Box{{X: 0.1, Y: 0.1, W: 0.8, H: 0.8}}, Page: []model.TplBg{{Lib: bg}}}
	look := lvLook(t, "main", &model.CoverTpl{Front: &front}, lvStep(2, 2, bg, model.Box{X: 0.1, Y: 0.1, W: 0.35, H: 0.8}))
	e := spEnv{g: buildLibVerify(t), files: files, look: look, all: append(append([]pfile{}, files...), look)}
	t.Run("лист из zip: все виды, baseline, items = sha256[:16], клетки — байты архива", e.sheet)
	t.Run("отбор по умолчанию — первые по манифесту: 8 фонов, 12 стикеров", e.caps)
	t.Run("-pick: свои записи по именам манифеста; неизвестное поле, имя, вид, повтор, >12 — отказ без файлов", e.pick)
	t.Run("плохие входы — ошибка вместо паники, в -out ничего", e.badInputs)
	t.Run("провал донора и сбой записи пары — лист не пишется, половины пары нет", e.pairAndDonor)
	t.Run("шумный лист не выходит за 384 КиБ: качество снижается", e.noisy)
	t.Run("сквозной: -sprite → library-pack --preview-only; архив и его sha256 не меняются", e.e2e)
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
