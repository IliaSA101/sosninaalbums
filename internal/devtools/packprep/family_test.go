package packprep

import (
	"image"
	"strings"
	"testing"
)

func TestFamilyOf(t *testing.T) {
	for src, want := range map[string]string{
		"school-frames/sf-leaves-corners-2-flux.jpg":       "sf-leaves",
		"school-frames/sf-leaves-diagonal-mystic.png":      "sf-leaves",
		"kindergarten-common/kg-fr-toys2-flux-1.jpg":       "kg-fr-toys2",
		"kindergarten-common/kg-fr-toys-abc-mystic-10.png": "kg-fr-toys",
		"kindergarten-common/kg-bg-rain-mystic-4.png":      "kg-bg-rain",
		"kindergarten-common/kg-div-flux-2.jpg":            "kg-div",
		"wedding/ns-frame-diagonal-lush-mystic.png":        "ns-frame-diagonal",
		"x/nb2-sheet-51708.png":                            "nb2-sheet-51708",
		"plain.png":                                        "plain",
	} {
		if got := familyOf(src); got != want {
			t.Errorf("%s: семейство %q, ждали %q", src, got, want)
		}
	}
}

func frameItem(src string) Item {
	return Item{Src: src, Mode: ModeFrame, Out: "frames/" + strings.TrimSuffix(src[strings.LastIndexByte(src, '/')+1:], ".png") + ".png", Name: "Рамка"}
}

// Больше двух источников одного семейства — ошибка; два — можно; повтор того же src, отбракованный
// src и явное family считаются верно.
func TestFamilyCap(t *testing.T) {
	three := baseRecipe(frameItem("f/sf-leaves-a.png"), frameItem("f/sf-leaves-b.png"), frameItem("f/sf-leaves-c.png"))
	errs := checkFamilies(three)
	if len(errs) != 1 || !strings.Contains(errs[0], "«sf-leaves»") || !strings.Contains(errs[0], "3 источников") {
		t.Fatalf("три из семейства: %v", errs)
	}
	two := baseRecipe(frameItem("f/sf-leaves-a.png"), frameItem("f/sf-leaves-b.png"), frameItem("f/sf-pencils-a.png"))
	if errs := checkFamilies(two); len(errs) != 0 {
		t.Fatalf("два из семейства: %v", errs)
	}
	three.Skip = map[string]string{"f/sf-leaves-c.png": "брак"}
	if errs := checkFamilies(three); len(errs) != 0 {
		t.Fatalf("отбракованный не считается: %v", errs)
	}
	three.Skip = nil
	three.Items[2].Family = "sf-ferns"
	if errs := checkFamilies(three); len(errs) != 0 {
		t.Fatalf("явное family: %v", errs)
	}
	// Один src в двух предметах (два способа обработки) — один источник семейства.
	dup := baseRecipe(frameItem("f/sf-leaves-a.png"), frameItem("f/sf-leaves-a.png"), frameItem("f/sf-leaves-b.png"))
	if errs := checkFamilies(dup); len(errs) != 0 {
		t.Fatalf("повтор src: %v", errs)
	}
}

// Через Run: ошибка семейства в отчёте, остальные предметы всё равно собраны.
func TestFamilyCapInRun(t *testing.T) {
	src := tempDir(t)
	for _, n := range []string{"a", "b", "c"} {
		img := canvas(120, 120, white)
		fillRect(img, image.Rect(10, 10, 110, 110), navy)
		fillRect(img, image.Rect(40, 40, 80, 80), white)
		writePNG(t, src+"/f/sf-leaves-"+n+".png", img)
	}
	rep, out := run(t, src, baseRecipe(frameItem("f/sf-leaves-a.png"), frameItem("f/sf-leaves-b.png"), frameItem("f/sf-leaves-c.png")))
	if rep.OK || !hasError(rep, "семейство «sf-leaves»") {
		t.Fatalf("ждали ошибку семейства: %v", rep.Errors)
	}
	if len(rep.Items) != 3 {
		t.Fatalf("предметы должны собираться и при ошибке: %d", len(rep.Items))
	}
	readOut(t, out, "_check/report.json")
}
