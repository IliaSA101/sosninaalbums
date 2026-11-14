package forms

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"
)

// Эталон Free (105, design §7): ключи и виды в порядке плиток на наборах 3–5 фото × геометрии
// (и «любые фото»), «ключ вид» по строкам — порядок часть контракта. После выпуска 0.24 не менять.
const goldenFreePath = "testdata/golden/free.sha"

func TestGoldenFree(t *testing.T) {
	sets := freeSets()
	var got []string
	for _, g := range append(append([]namedGeom{}, geoms...), geomsAny()...) {
		for _, name := range sortedNames(sets) {
			var lines []string
			cs, hidden := FreeFold(sets[name], g.p)
			for _, c := range cs {
				lines = append(lines, c.Key+" "+string(c.Form))
			}
			h := sha256.Sum256([]byte(strings.Join(lines, "\n")))
			got = append(got, fmt.Sprintf("%s/%s %d скрыто %d %s", g.name, name, len(lines), hidden, hex.EncodeToString(h[:8])))
		}
	}
	checkGoldenForms(t, goldenFreePath, got)
}
