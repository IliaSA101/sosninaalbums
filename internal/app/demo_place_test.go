package app

import (
	"bytes"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/app/libpacks"
	"github.com/IliaSA101/sosninaalbums/internal/decor"
	"github.com/IliaSA101/sosninaalbums/internal/demopack"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// demoStickerPNG — малый PNG с прозрачностью для предмета примера.
func demoStickerPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 20, 20))
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+3] = 200, 255
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// demoFileApp — App с малым zip примера в data/ рядом с «программой»; SetDemo уже по нему.
func demoFileApp(t *testing.T) (*App, *netLib) {
	t.Helper()
	a, l := netApp(t, true)
	z := packZip(t, demopack.ID, demopack.Version, pfile{name: "stickers/a.png", kind: "sticker", data: demoStickerPNG(t)})
	data, err := os.ReadFile(z)
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(testkit.TempDir(t), "data")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, demopack.File)
	if err := os.WriteFile(p, data, 0o600); err != nil {
		t.Fatal(err)
	}
	f := demoAt(data, p)
	a.demoFn = func() libpacks.Demo { return makeDemo(f, false) }
	a.openDecor() // SetDemo по новому примеру
	return a, l
}

// TestAppPlaceDemo — шов placeDemo: малый zip в data/ рядом с «программой» ставится без сети и
// вопроса, origin demo, одно library:changed без названий; в окне «Наборы» нет строки «Скачать»;
// удаление пишет пропуск, «Вернуть» (ResetLibrarySkip) ставит пример снова и отвечает demoBack.
func TestAppPlaceDemo(t *testing.T) {
	a, l := demoFileApp(t)
	ev := captureEvents(a)

	a.placeDemo()
	list, err := a.LibraryPacks()
	if err != nil {
		t.Fatal(err)
	}
	var got *PackInfo
	for i, pk := range list.Packs {
		if pk.ID == demopack.ID {
			got = &list.Packs[i]
		}
	}
	if got == nil || got.Origin != decor.OriginDemo || !got.Removable || len(list.Available) != 0 || list.Net.DemoOffer {
		t.Fatalf("после placeDemo: %+v %+v", got, list)
	}
	if ch := ev.of("library:changed"); len(ch) != 1 {
		t.Errorf("library:changed: %v", ch)
	}
	if l.hits.Load() != 0 {
		t.Errorf("пример из файла пошёл в сеть: %d", l.hits.Load())
	}

	// Удалить → пропуск; «Вернуть» → пример снова и demoBack.
	if _, err := a.RemovePack(demopack.ID); err != nil {
		t.Fatal(err)
	}
	if list, _ = a.LibraryPacks(); !list.Net.DemoSkipped || list.Net.Skipped != 1 {
		t.Fatalf("после удаления: %+v", list.Net)
	}
	n, err := a.ResetLibrarySkip()
	if err != nil || !n.DemoBack || n.Skipped != 0 || n.DemoSkipped {
		t.Fatalf("Вернуть: %v %+v", err, n)
	}
	if list, _ = a.LibraryPacks(); !hasPackInfo(list.Packs, demopack.ID) {
		t.Error("пример не вернулся")
	}
}

func hasPackInfo(packs []PackInfo, id string) bool {
	for _, p := range packs {
		if p.ID == id {
			return true
		}
	}
	return false
}

// placeDemo ждёт конец startup: domReady может обогнать его, а библиотека и cfgDir готовы позже.
// Остановка программы отпускает ожидание без установки.
func TestAppPlaceDemoWaitsReady(t *testing.T) {
	a, _ := demoFileApp(t)
	a.ready = make(chan struct{}) // как после NewApp: startup ещё идёт
	done := make(chan struct{})
	go func() { a.placeDemo(); close(done) }()
	select {
	case <-done:
		t.Fatal("placeDemo вернулся до конца startup")
	case <-time.After(50 * time.Millisecond):
	}
	if list, _ := a.LibraryPacks(); hasPackInfo(list.Packs, demopack.ID) {
		t.Fatal("пример встал до конца startup")
	}
	close(a.ready)
	select {
	case <-done:
	case <-time.After(testkit.Limit(30 * time.Second)): // «не завис»: под нагрузкой полного прогона установка примера идёт дольше 5 с
		t.Fatal("placeDemo не вернулся после startup")
	}
	if list, _ := a.LibraryPacks(); !hasPackInfo(list.Packs, demopack.ID) {
		t.Error("пример не встал после startup")
	}
}

func TestAppPlaceDemoStopReleases(t *testing.T) {
	a, _ := demoFileApp(t)
	a.ready = make(chan struct{})
	done := make(chan struct{})
	go func() { a.placeDemo(); close(done) }()
	a.stopBackground()
	select {
	case <-done:
	case <-time.After(testkit.Limit(5 * time.Second)):
		t.Fatal("placeDemo не вернулся после остановки")
	}
	if list, _ := a.LibraryPacks(); hasPackInfo(list.Packs, demopack.ID) {
		t.Error("пример встал при остановке")
	}
}
