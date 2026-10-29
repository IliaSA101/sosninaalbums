package faces

import (
	_ "embed" // веса модели вшиты в exe
	"fmt"
	"sync"
)

// Version — ключ кэша точек: меняется вместе с моделью, подготовкой входа, порогами и
// фильтром. Старые точки после смены версии не используются.
const Version = "yunet-2023mar-1"

// modelFile — YuNet 2023mar из OpenCV Zoo (232 589 байт, MIT, © 2020 Shiqi Yu). Лицензия лежит
// рядом (LICENSE-yunet); sha256 проверяет TestModelPinned.
//
//go:embed face_detection_yunet_2023mar.onnx
var modelFile []byte

var (
	netOnce sync.Once
	theNet  *net
	netErr  error
)

// loadNet разбирает и компилирует модель один раз на процесс (≈ 0,5 мс); дальше сеть только читается.
func loadNet() (*net, error) {
	netOnce.Do(func() {
		var g *onnxGraph
		if g, netErr = parseONNX(modelFile); netErr != nil {
			return
		}
		theNet, netErr = compileNet(g)
	})
	return theNet, netErr
}

// Ready — модель разобрана и скомпилирована, а если нет — почему. Первый вызов загружает сеть
// (≈ 0,5 мс), дальше возвращает тот же ответ; Detect при сбое отдаёт nil без причины, а журнал
// берёт её отсюда.
func Ready() error {
	if _, err := loadNet(); err != nil {
		return fmt.Errorf("faces: модель не загрузилась: %w", err)
	}
	return nil
}
