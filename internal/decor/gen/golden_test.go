package gen

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"flag"
	"image"
	"image/png"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"testing"
)

// Эталон генераторов (решение №90): выпущенный узор не меняется — хэш пикселей на двух
// размерах (плитка и экран; второй идёт через мягкий слой). Перезапись — только для нового
// ключа: go test ./internal/decor/gen -run TestGenGolden -update (или SA_GOLDEN_UPDATE=1).
// SA_GEN_DUMP=<папка> — PNG всех эталонных картинок для просмотра глазами.
var updateGolden = flag.Bool("update", false, "дописать эталоны testdata/golden.json")

const goldenPath = "testdata/golden.json"

// goldenMM — область эталона; ширины — плитка (0,8 px/мм) и экран (6 px/мм, мягкий слой).
var (
	goldenMM = [2]float64{200, 140}
	goldenPx = [2]int{160, 1200}
)

// goldenSpec — эталонный Spec ключа: цвета генератора, сид 7.
func goldenSpec(kind string) Spec { return Spec{Kind: kind, Seed: 7} }

// goldenHash — первые 8 байт sha256 размера и пикселей (не PNG: кодер может смениться с Go).
func goldenHash(img *image.RGBA) string {
	h := sha256.New()
	_ = binary.Write(h, binary.LittleEndian, [2]int32{int32(img.Rect.Dx()), int32(img.Rect.Dy())})
	h.Write(img.Pix)
	return hex.EncodeToString(h.Sum(nil)[:8])
}

// goldenArts — хэши стикеров: ответ /media/gen/art в своей пропорции (длинная сторона —
// goldenPx); растяжимые ещё и в рамках 1:1 и 4:1 — рисунок от короткой стороны не растягивается
// (решение №136). Ключ — «вид» или «вид@пропорция».
func goldenArts(t *testing.T, dump string) map[string][2]string {
	t.Helper()
	got := map[string][2]string{}
	for _, m := range Arts() {
		for _, a := range goldenAspects(m) {
			key := m.Kind
			if a != 0 {
				key += "@" + strconv.FormatFloat(a, 'f', -1, 64)
			}
			var hs [2]string
			for i, px := range goldenPx {
				img := Request{Art: true, Spec: Spec{Kind: m.Kind, Seed: 7, Aspect: a}, Px: px}.Draw()
				hs[i] = goldenHash(img)
				if dump != "" {
					writePNG(t, filepath.Join(dump, key+"-"+strconv.Itoa(px)+".png"), img)
				}
			}
			got[key] = hs
		}
	}
	return got
}

// goldenAspects — рамки эталона стикера: 0 — своя пропорция; у растяжимого ещё 1 и 4.
func goldenAspects(m Meta) []float64 {
	if m.Stretch {
		return []float64{0, 1, 4}
	}
	return []float64{0}
}

func renderGolden(kind string, px int) *image.RGBA {
	k := float64(px) / goldenMM[0]
	img := image.NewRGBA(image.Rect(0, 0, px, int(goldenMM[1]*k+0.5)))
	DrawPattern(img, img.Rect, goldenSpec(kind), k, 0)
	return img
}

// TestGenGolden — каждый узор сборки совпадает с эталоном побитово на обоих размерах; ключ
// эталона без генератора — тоже провал (выпущенный генератор не удаляют).
func TestGenGolden(t *testing.T) {
	update := *updateGolden || os.Getenv("SA_GOLDEN_UPDATE") == "1"
	// На arm64 компилятор вправе сливать x*y+z в FMA — последние биты координат другие.
	if runtime.GOARCH != "amd64" && os.Getenv("SA_GOLDEN_ANYARCH") != "1" && !update {
		t.Skip("эталон снят на amd64")
	}
	want := loadGolden(t, update)
	dump := os.Getenv("SA_GEN_DUMP")
	got := map[string][2]string{}
	for _, m := range Patterns() {
		var hs [2]string
		for i, px := range goldenPx {
			img := renderGolden(m.Kind, px)
			hs[i] = goldenHash(img)
			if dump != "" {
				writePNG(t, filepath.Join(dump, m.Kind+"-"+strconv.Itoa(px)+".png"), img)
			}
		}
		got[m.Kind] = hs
	}
	maps.Copy(got, goldenArts(t, dump))
	if update {
		for k, v := range want {
			if _, ok := got[k]; !ok {
				got[k] = v // ключ без генератора (текстура не вшита) — эталон остаётся
			}
		}
		data, _ := json.MarshalIndent(got, "", "  ")
		if err := os.WriteFile(goldenPath, append(data, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	kinds := make([]string, 0, len(got))
	for k := range got {
		kinds = append(kinds, k)
	}
	sort.Strings(kinds)
	for _, k := range kinds {
		w, ok := want[k]
		if !ok {
			t.Errorf("%s: нет эталона — новый ключ: -update", k)
			continue
		}
		if w != got[k] {
			t.Errorf("%s: рисунок изменился (%v, эталон %v) — выпущенный генератор не меняют, правка — новый ключ", k, got[k], w)
		}
	}
	for k := range want {
		if _, ok := got[k]; !ok && !isPaper(k) {
			t.Errorf("%s: эталон есть, генератора нет — выпущенный ключ удалён", k)
		}
	}
}

func loadGolden(t *testing.T, update bool) map[string][2]string {
	t.Helper()
	want := map[string][2]string{}
	data, err := os.ReadFile(goldenPath)
	if err != nil {
		if !update {
			t.Fatal(err)
		}
		return want
	}
	if err := json.Unmarshal(data, &want); err != nil {
		t.Fatalf("%s: %v", goldenPath, err)
	}
	return want
}

// isPaper — ключ текстуры: её генератор есть, только если файл вшит (O1).
func isPaper(kind string) bool {
	for _, g := range patternGens {
		if g.meta.Kind == kind {
			return g.meta.Paper
		}
	}
	return false
}

func writePNG(t *testing.T, path string, img image.Image) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}
