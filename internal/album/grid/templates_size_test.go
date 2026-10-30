package grid

import (
	"bytes"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// minTemplatesHeadroom — запас worst-case файла шаблонов до MaxTemplatesFile, байт. Предел 1 МБ
// не поднимаем (прежние программы уводят больший файл в .bad), поэтому новое поле у места или
// виньетки сначала проверяется здесь: просело ниже запаса — формат сжимать (как в 0.21: общее
// поле один раз на шаблон), а не терять шаблоны с хвоста списка при чтении.
const minTemplatesHeadroom = 10 << 10

// worstTemplates — 200 шаблонов виньетки по 64 места: овал, поворот, все знаки доли, имя
// предельной длины (эти поля в худшем случае заполнены у каждого места), без длинных подписей
// (они уже сейчас выводят случай за предел и режутся с хвоста — TestParseTemplatesFitsFile).
func worstTemplates() []model.Template {
	list := make([]model.Template, model.MaxTemplates)
	for i := range list {
		tp := vigTplOf(model.MaxPlaces, 1)
		tp.Name = strings.Repeat("я", model.MaxTemplateName)
		for j := range tp.Boxes {
			tp.Boxes[j].X += 1e-5
			tp.Boxes[j].Rot = 12.34567
			tp.Boxes[j].Shape = model.ShapeOval
		}
		list[i] = tp
	}
	return model.RoundTemplates(list)
}

// Худший файл шаблонов виньеток укладывается в предел с запасом, все 200 шаблонов проходят круг
// Marshal → Parse без потерь с хвоста, а файл больше предела читается как битый.
func TestTemplatesWorstCaseHeadroom(t *testing.T) {
	list := worstTemplates()
	data, err := model.MarshalTemplates(model.ClampTemplates(list))
	if err != nil {
		t.Fatal(err)
	}
	if room := model.MaxTemplatesFile - len(data); room < minTemplatesHeadroom {
		t.Fatalf("файл %d байт из %d: запас %d < %d — сжать формат до нового поля", len(data), model.MaxTemplatesFile, room, minTemplatesHeadroom)
	}
	got, _, err := model.ParseTemplates(data)
	if err != nil || len(got) != len(list) {
		t.Fatalf("прочитано %d из %d: %v", len(got), len(list), err)
	}
	for i := range got {
		if got[i].ID != list[i].ID || len(got[i].Boxes) != model.MaxPlaces || got[i].Boxes[0].Shape != model.ShapeOval {
			t.Fatalf("шаблон %d изменился при круге: %+v", i, got[i].Name)
		}
	}
	// Предел: к худшему файлу дописаны пробелы сверх 1 МБ (JSON остаётся валидным — отказать может только предел) —
	// ParseTemplates такой не читает (битый), а не режет молча.
	huge := append([]byte(nil), data...)
	huge = append(huge, bytes.Repeat([]byte(" "), model.MaxTemplatesFile-len(data)+1)...)
	if _, _, err := model.ParseTemplates(huge[:model.MaxTemplatesFile]); err != nil {
		t.Fatalf("файл ровно в пределе не прочитан: %v", err)
	}
	if _, _, err := model.ParseTemplates(huge); err == nil {
		t.Error("файл больше предела прочитан")
	}
}
