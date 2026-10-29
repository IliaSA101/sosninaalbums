package decor

import "testing"

// DemoCovered: пример покрыт, только когда каждая пара (вид, ключ) его предметов есть у стоящих
// доноров; пример без доноров, без SetDemo и пустая библиотека — не покрыты.
func TestDemoCovered(t *testing.T) {
	s, _ := openTemp(t)
	a, b, c := renderPNG(t, 20, 20), renderPNG(t, 21, 20), renderPNG(t, 22, 20)
	s.SetDemo(demoID, []string{"donor-one", "donor-two"})
	if s.DemoCovered() {
		t.Error("пустая библиотека покрыта")
	}
	installDemo(t, s, pit{"stickers/a.png", a}, pit{"stickers/b.png", b}, pit{"stickers/c.png", c})
	if s.DemoCovered() {
		t.Error("пример без доноров покрыт")
	}
	m, src := testPack("donor-one", 1, pit{"stickers/a.png", a}, pit{"stickers/b.png", b})
	installOrigin(t, s, OriginNet, m, src)
	if s.DemoCovered() {
		t.Error("покрыт двумя из трёх предметов")
	}
	m, src = testPack("donor-two", 1, pit{"stickers/c.png", c})
	installOrigin(t, s, OriginNet, m, src)
	if !s.DemoCovered() {
		t.Error("три из трёх не покрывают")
	}
	s.SetDemo("", nil) // примера нет
	if s.DemoCovered() {
		t.Error("без SetDemo покрыт")
	}
	s.SetDemo(demoID, nil) // доноров нет
	if s.DemoCovered() {
		t.Error("без доноров покрыт")
	}
}

// Тот же ключ, другой вид: байты одни, но донор держит их фоном, а пример — стикером; пара
// (вид, ключ) не совпадает — пример не покрыт, удаление унесло бы плитку стикера.
func TestDemoCoveredSameKeyOtherKind(t *testing.T) {
	s, _ := openTemp(t)
	a := renderPNG(t, 20, 20)
	s.SetDemo(demoID, []string{"donor-one", "donor-two"})
	installDemo(t, s, pit{"stickers/a.png", a})
	m, src := testPack("donor-one", 1, pit{"bg/a.png", a})
	installOrigin(t, s, OriginNet, m, src)
	if s.DemoCovered() {
		t.Error("фон покрыл стикер с тем же ключом")
	}
	m, src = testPack("donor-two", 1, pit{"stickers/a.png", a})
	installOrigin(t, s, OriginNet, m, src)
	if !s.DemoCovered() {
		t.Error("тот же вид и ключ не покрывают")
	}
}
