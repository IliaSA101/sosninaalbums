package faces

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"strings"
	"sync"
	"testing"
)

func sha(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

// TestModelPinned: вшиты именно проверенные веса YuNet 2023mar (sha256 из OpenCV Zoo), а рядом
// лежит их лицензия MIT — без неё веса в exe нести нельзя (THIRD_PARTY_NOTICES, design §9).
func TestModelPinned(t *testing.T) {
	const want = "8f2383e4dd3cfbb4553ea8718107fc0423210dc964f9f4280604804ed2552fa4"
	if len(modelFile) != 232589 {
		t.Errorf("размер модели %d, ждали 232589", len(modelFile))
	}
	if got := sha(modelFile); got != want {
		t.Errorf("sha256 модели %s, ждали %s: менять веса — новая спека", got, want)
	}
	lic, err := os.ReadFile("LICENSE-yunet")
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{"MIT License", "Copyright (c) 2020 Shiqi Yu", "Permission is hereby granted"} {
		if !strings.Contains(string(lic), s) {
			t.Errorf("в LICENSE-yunet нет %q", s)
		}
	}
}

// TestReady: вшитая модель грузится; при сбое Ready называет причину, и она не меняется от
// вызова к вызову, а Detect отдаёт nil без паники. Состояние загрузки подменяется на время теста
// (тесты пакета не параллельны) и возвращается.
func TestReady(t *testing.T) {
	if err := Ready(); err != nil {
		t.Fatalf("вшитая модель не загрузилась: %v", err)
	}
	saved, savedNet, savedErr := modelFile, theNet, netErr
	defer func() {
		modelFile, theNet, netErr, netOnce = saved, savedNet, savedErr, sync.Once{}
		netOnce.Do(func() {}) // загрузка уже сделана выше: Once не должен её повторять
	}()
	for name, bad := range map[string][]byte{"пусто": nil, "мусор": []byte("не onnx"), "обрезана": saved[:len(saved)/3]} {
		modelFile, theNet, netErr, netOnce = bad, nil, nil, sync.Once{}
		err := Ready()
		if err == nil || !strings.Contains(err.Error(), "модель не загрузилась") {
			t.Errorf("%s: Ready = %v, ждали причину сбоя", name, err)
		}
		if again := Ready(); again == nil || again.Error() != err.Error() {
			t.Errorf("%s: повторный Ready = %v, был %v", name, again, err)
		}
		if fs := Detect(noiseRGBA(64, 64)); fs != nil {
			t.Errorf("%s: Detect без модели нашёл %v", name, fs)
		}
	}
}
