package update

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/decor/fetch"
)

func checkOf(t *testing.T, f *feed, cur, seen string) (Result, error) {
	t.Helper()
	return Check(context.Background(), f.client(), cur, seen)
}

// Новее текущей — предложение с установщиком и текстом; запрос один.
func TestCheckOffer(t *testing.T) {
	f := newFeed(t)
	_, file := setup("0.25.1", MinSetup)
	f.serveManifest(manifestBody(t, "0.25.1", &file, nil))
	res, err := checkOf(t, f, "0.25.0", "")
	if err != nil || res.Offer == nil || res.Offer.Version != "0.25.1" || res.Offer.Setup == nil || *res.Offer.Setup != file ||
		res.Offer.Page || len(res.Offer.Notes) != 2 || res.Seen != "0.25.1" || res.Rollback {
		t.Fatalf("предложение: %v %+v", err, res)
	}
	if n := f.hits.Load(); n != 1 {
		t.Errorf("запросов %d", n)
	}
}

// Версия ≤ своей — «последняя»; Seen при этом растёт до версии файла.
func TestCheckLatest(t *testing.T) {
	f := newFeed(t)
	_, file := setup("0.25.1", MinSetup)
	f.serveManifest(manifestBody(t, "0.25.1", &file, nil))
	for _, cur := range []string{"0.25.1", "0.25.2", "1.0.0"} {
		res, err := checkOf(t, f, cur, "")
		if err != nil || res.Offer != nil || res.Rollback || res.Seen != "0.25.1" {
			t.Errorf("cur %s: %v %+v", cur, err, res)
		}
	}
	// Своя версия не версия (dev, пусто) — предлагать нечего.
	for _, cur := range []string{"dev", ""} {
		if res, err := checkOf(t, f, cur, ""); err != nil || res.Offer != nil {
			t.Errorf("cur %q: %v %+v", cur, err, res)
		}
	}
}

// Подпись: чужой ключ, порча body, запасной ключ в списке — принят (design §5.3).
func TestCheckSignature(t *testing.T) {
	f := newFeed(t)
	_, file := setup("0.25.1", MinSetup)
	body := manifestBody(t, "0.25.1", &file, nil)
	other, otherPriv, _ := ed25519.GenerateKey(rand.Reader)

	f.serveRaw(envelope(otherPriv, body)) // подписан чужим ключом
	if _, err := checkOf(t, f, "0.25.0", ""); !errors.Is(err, ErrVerify) {
		t.Errorf("чужой ключ: %v", err)
	}
	// Запасной ключ вторым в списке: подпись любым из двух.
	g := newFeed(t)
	g.serveRaw(envelope(otherPriv, body))
	src := g.source()
	src.Keys = []ed25519.PublicKey{g.pub, other}
	res, err := Check(context.Background(), New(src), "0.25.0", "")
	if err != nil || res.Offer == nil {
		t.Errorf("запасной ключ: %v %+v", err, res)
	}
	// Порча: подпись на одних байтах, body — другие (версия поправлена на новее).
	h := newFeed(t)
	env := envelope(h.priv, body)
	forged := envelope(h.priv, manifestBody(t, "9.9.9", &file, nil))
	h.serveRaw(swapBody(t, env, forged))
	if _, err := checkOf(t, h, "0.25.0", ""); !errors.Is(err, ErrVerify) {
		t.Errorf("порча body: %v", err)
	}
	// Ключей нет вовсе (выпуск до заполнения keys.go) — отказ, а не пропуск проверки.
	src = f.source()
	src.Keys = nil
	f.serveManifest(body)
	if _, err := Check(context.Background(), New(src), "0.25.0", ""); !errors.Is(err, ErrVerify) {
		t.Errorf("без ключей: %v", err)
	}
}

// Откат: файл ниже виденной версии — «последняя», Seen не падает; равный виденной — обычная проверка.
func TestCheckNoRollback(t *testing.T) {
	f := newFeed(t)
	_, file := setup("0.25.1", MinSetup)
	f.serveManifest(manifestBody(t, "0.25.1", &file, nil))
	res, err := checkOf(t, f, "0.24.0", "0.26.0")
	if err != nil || res.Offer != nil || !res.Rollback || res.Seen != "0.26.0" {
		t.Errorf("старый файл при виденной 0.26.0: %v %+v", err, res)
	}
	res, err = checkOf(t, f, "0.24.0", "0.25.1")
	if err != nil || res.Offer == nil || res.Rollback || res.Seen != "0.25.1" {
		t.Errorf("равный виденной: %v %+v", err, res)
	}
	// Мусор в config.json вместо версии — как «не видели».
	res, err = checkOf(t, f, "0.24.0", "скоро")
	if err != nil || res.Offer == nil || res.Seen != "0.25.1" {
		t.Errorf("seen мусор: %v %+v", err, res)
	}
}

// minAuto новее своей версии и выпуск без установщика Windows — только страница выпуска.
func TestCheckMinAuto(t *testing.T) {
	f := newFeed(t)
	_, file := setup("0.26.0", MinSetup)
	f.serveManifest(manifestBody(t, "0.26.0", &file, map[string]any{"minAuto": "0.25.0"}))
	if res, err := checkOf(t, f, "0.24.0", ""); err != nil || res.Offer == nil || !res.Offer.Page || res.Offer.Setup != nil {
		t.Errorf("minAuto новее: %v %+v", err, res.Offer)
	}
	if res, err := checkOf(t, f, "0.25.0", ""); err != nil || res.Offer == nil || res.Offer.Page || res.Offer.Setup == nil {
		t.Errorf("minAuto равен: %v %+v", err, res.Offer)
	}
	g := newFeed(t)
	g.serveManifest(manifestBody(t, "0.26.0", nil, nil))
	if res, err := checkOf(t, g, "0.25.0", ""); err != nil || res.Offer == nil || !res.Offer.Page || res.Offer.Setup != nil {
		t.Errorf("без Windows: %v %+v", err, res.Offer)
	}
}

// Ошибки словами обновления: формат новее, 404 без слов о библиотеке, нет связи.
func TestCheckErrors(t *testing.T) {
	f := newFeed(t)
	f.serveManifest(manifestBody(t, "0.26.0", nil, map[string]any{"format": 2}))
	if _, err := checkOf(t, f, "0.25.0", ""); !errors.Is(err, ErrNewerFormat) {
		t.Errorf("формат новее: %v", err)
	}
	g := newFeed(t) // манифеста нет — 404
	if _, err := checkOf(t, g, "0.25.0", ""); !errors.Is(err, ErrNoFile) {
		t.Errorf("404: %v", err)
	}
	h := newFeed(t)
	h.fail.Store(true)
	if _, err := checkOf(t, h, "0.25.0", ""); err == nil || errors.Is(err, ErrVerify) {
		t.Errorf("502: %v", err)
	}
	// Конверт больше предела — не читается целиком.
	big := newFeed(t)
	big.serveRaw(make([]byte, maxEnvelope+10))
	if _, err := checkOf(t, big, "0.25.0", ""); !errors.Is(err, ErrVerify) {
		t.Errorf("большой конверт: %v", err)
	}
	// Отмена не превращается в «не прошло проверку».
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Check(ctx, f.client(), "0.25.0", ""); !errors.Is(err, ErrCancelled) {
		t.Errorf("отмена: %v", err)
	}
	if humanize(nil) != nil || humanize(fetch.ErrOffline) != fetch.ErrOffline {
		t.Error("humanize: нет ошибки и сеть — как есть")
	}
}
