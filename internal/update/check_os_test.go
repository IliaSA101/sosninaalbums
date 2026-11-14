package update

import (
	"reflect"
	"testing"
)

// osFeed — лента с манифестом версии ver и правилами minOS/lastOn (JSON поля, "" — нет поля).
func osFeed(t *testing.T, ver, minOS, lastOn string) *feed {
	t.Helper()
	f := newFeed(t)
	_, file := setup(ver, MinSetup)
	rest := map[string]any{}
	if minOS != "" {
		rest["minOS"] = rawJSON(minOS)
	}
	if lastOn != "" {
		rest["lastOn"] = rawJSON(lastOn)
	}
	f.serveManifest(manifestBody(t, ver, &file, rest))
	return f
}

type rawJSON string

func (r rawJSON) MarshalJSON() ([]byte, error) { return []byte(r), nil }

var mac = func(v string) System { return System{GOOS: "darwin", Version: v} }

// Система ниже, равна и выше minOS; ключ другой ОС; неизвестная версия системы; откат.
func TestCheckMinOS(t *testing.T) {
	f := osFeed(t, "0.43.0", `{"darwin":"13.0","windows":"10.0.22000"}`, "")
	// Ниже: предложения нет, Blocked/Need заполнены, Seen — версия файла.
	res, err := checkSys(t, f, "0.42.0", "", mac("12.4"))
	if err != nil || res.Offer != nil || res.Blocked != "0.43.0" || res.Need != "13.0" || res.Seen != "0.43.0" || res.OSUnknown || res.Rollback {
		t.Errorf("ниже: %v %+v", err, res)
	}
	// Равна и выше: предложение как обычно, установщик на месте.
	for _, v := range []string{"13.0", "13", "13.0.1", "14.2", "26.0"} {
		res, err := checkSys(t, f, "0.42.0", "", mac(v))
		if err != nil || res.Offer == nil || res.Offer.Version != "0.43.0" || res.Offer.Page || res.Offer.Setup == nil || res.Blocked != "" || res.Need != "" {
			t.Errorf("система %s: %v %+v", v, err, res)
		}
	}
	// Ключ другой ОС: на Linux и Windows 11 правило Mac не действует, а Windows 10 ниже своего минимума.
	if res, _ := checkSys(t, f, "0.42.0", "", System{GOOS: "linux", Version: "5.4"}); res.Offer == nil || res.Blocked != "" || res.OSUnknown {
		t.Errorf("linux: %+v", res)
	}
	if res, _ := checkSys(t, f, "0.42.0", "", System{GOOS: "windows", Version: "10.0.22631"}); res.Offer == nil || res.Blocked != "" {
		t.Errorf("windows 11: %+v", res)
	}
	if res, _ := checkSys(t, f, "0.42.0", "", System{GOOS: "windows", Version: "10.0.19045"}); res.Offer != nil || res.Blocked != "0.43.0" || res.Need != "10.0.22000" {
		t.Errorf("windows 10: %+v", res)
	}
	// Версия системы неизвестна: предложение как прежде, OSUnknown — только при своём minOS.
	for _, sys := range []System{mac(""), mac("мусор"), {GOOS: "windows"}} {
		if res, _ := checkSys(t, f, "0.42.0", "", sys); res.Offer == nil || !res.OSUnknown || res.Blocked != "" {
			t.Errorf("неизвестна %+v: %+v", sys, res)
		}
	}
	if res, _ := checkSys(t, f, "0.42.0", "", System{GOOS: "linux"}); res.Offer == nil || res.OSUnknown {
		t.Errorf("linux без версии: %+v", res)
	}
}

// Края: без правил, выпуск не новее cur, откат, неверные поля.
func TestCheckMinOSEdges(t *testing.T) {
	f := osFeed(t, "0.43.0", `{"darwin":"13.0","windows":"10.0.22000"}`, "")
	// Без правил и без системы вовсе — как раньше.
	g := osFeed(t, "0.43.0", "", "")
	if res, _ := checkSys(t, g, "0.42.0", "", mac("12.4")); res.Offer == nil || res.Blocked != "" || res.OSUnknown {
		t.Errorf("без правил: %+v", res)
	}
	if res, _ := checkSys(t, f, "0.42.0", "", System{}); res.Offer == nil || res.OSUnknown || res.Blocked != "" {
		t.Errorf("система пуста: %+v", res)
	}
	// Выпуск не новее текущей: блокировки нет, это просто «последняя».
	for _, cur := range []string{"0.43.0", "0.44.0", "dev"} {
		if res, _ := checkSys(t, f, cur, "", mac("12.4")); res.Offer != nil || res.Blocked != "" || res.OSUnknown {
			t.Errorf("cur %s: %+v", cur, res)
		}
	}
	// Откат важнее блокировки: файл старее виденной версии — Rollback без Blocked, Seen не падает.
	res, err := checkSys(t, f, "0.42.0", "0.50.0", mac("12.4"))
	if err != nil || !res.Rollback || res.Blocked != "" || res.Offer != nil || res.Seen != "0.50.0" {
		t.Errorf("откат: %v %+v", err, res)
	}
	// Виденная равна версии файла — блокировка работает.
	if res, _ = checkSys(t, f, "0.42.0", "0.43.0", mac("12.4")); res.Rollback || res.Blocked != "0.43.0" {
		t.Errorf("seen равна: %+v", res)
	}
	// Неверное поле чужой ОС: Skipped в итоге, обновление не блокируется.
	h := osFeed(t, "0.43.0", `{"darwin":"13.0","windows":"плохо"}`, "")
	res, err = checkSys(t, h, "0.42.0", "", mac("13.0"))
	if err != nil || res.Offer == nil || !reflect.DeepEqual(res.Skipped, []string{"minOS.windows"}) {
		t.Errorf("чужое поле: %v %+v", err, res)
	}
	// Неверное поле своей ОС — ErrManifest.
	if _, err := checkSys(t, h, "0.42.0", "", System{GOOS: "windows", Version: "10.0.22631"}); err == nil {
		t.Error("неверное поле своей ОС прошло")
	}
}

// Ступень lastOn: наименьший порог выше системы; новее cur — предложение-страница.
func TestCheckLadder(t *testing.T) {
	f := osFeed(t, "0.50.0", `{"darwin":"14.0"}`, `{"darwin":{"14.0":"0.47.0","13.0":"0.42.0"}}`)
	// Mac 12.4 при выпуске для 14: версия для 12 (порог 13), а не для 13.
	res, err := checkSys(t, f, "0.40.0", "", mac("12.4"))
	if err != nil || res.Offer == nil || res.Offer.Version != "0.42.0" || !res.Offer.Page || res.Offer.Setup != nil || res.Offer.Notes == nil ||
		res.Blocked != "0.50.0" || res.Need != "14.0" || res.Seen != "0.50.0" {
		t.Fatalf("Mac 12.4: %v %+v %+v", err, res, res.Offer)
	}
	// Mac 13.5 — последняя для 13 (порог 14).
	if res, _ := checkSys(t, f, "0.40.0", "", mac("13.5")); res.Offer == nil || res.Offer.Version != "0.47.0" || !res.Offer.Page || res.Blocked != "0.50.0" {
		t.Errorf("Mac 13.5: %+v %+v", res, res.Offer)
	}
	// Mac 14 и новее — обычное предложение.
	if res, _ := checkSys(t, f, "0.40.0", "", mac("14.0")); res.Offer == nil || res.Offer.Version != "0.50.0" || res.Offer.Page || res.Blocked != "" {
		t.Errorf("Mac 14: %+v %+v", res, res.Offer)
	}
	// Ступень равна и старее cur — предложения нет, остаётся «последняя для вашей системы».
	for _, cur := range []string{"0.42.0", "0.42.1", "0.45.0"} {
		if res, _ := checkSys(t, f, cur, "", mac("12.4")); res.Offer != nil || res.Blocked != "0.50.0" || res.Need != "14.0" {
			t.Errorf("cur %s на 12.4: %+v %+v", cur, res, res.Offer)
		}
	}
	// Для 13.5 ступень 0.47.0 против cur 0.47.0 и новее — без предложения; 0.46.9 — предложение.
	if res, _ := checkSys(t, f, "0.47.0", "", mac("13.5")); res.Offer != nil || res.Blocked != "0.50.0" {
		t.Errorf("cur 0.47.0 на 13.5: %+v", res)
	}
	if res, _ := checkSys(t, f, "0.46.9", "", mac("13.5")); res.Offer == nil || res.Offer.Version != "0.47.0" {
		t.Errorf("cur 0.46.9 на 13.5: %+v", res)
	}
	// Система между верхней ступенью и minOS, ступени выше нет — без предложения.
	g := osFeed(t, "0.50.0", `{"darwin":"15.0"}`, `{"darwin":{"14.0":"0.47.0"}}`)
	if res, _ := checkSys(t, g, "0.40.0", "", mac("14.5")); res.Offer != nil || res.Blocked != "0.50.0" || res.Need != "15.0" {
		t.Errorf("система выше ступеней: %+v", res)
	}
	// Откат всё ещё важнее блокировки и ступени.
	if res, _ := checkSys(t, f, "0.40.0", "0.60.0", mac("12.4")); !res.Rollback || res.Offer != nil || res.Blocked != "" {
		t.Errorf("откат: %+v", res)
	}
	// Версия системы неизвестна — предложение самой новой версии, лестница не применяется.
	if res, _ := checkSys(t, f, "0.40.0", "", mac("")); res.Offer == nil || res.Offer.Version != "0.50.0" || res.Offer.Page || !res.OSUnknown {
		t.Errorf("система неизвестна: %+v", res)
	}
}

func checkSys(t *testing.T, f *feed, cur, seen string, sys System) (Result, error) {
	t.Helper()
	return Check(t.Context(), f.client(), cur, seen, sys)
}
