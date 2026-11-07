package app

// Подсчёт папки перед тихим добавлением (150, design §4.5): сам обход — agentplaces.CountQuiet; здесь только
// снимок подмен App (st_dev, сетевой том, пауза, облачная заглушка) и срок.

import (
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/app/agentplaces"
)

var (
	quietTimeout = 1500 * time.Millisecond
	quietPause   = func() {}  // перед каждой порцией (тест: зависший диск)
	quietCloud   = entryCloud // запись — облачная заглушка (по ОС)
)

// countQuietCap считает папку root; capPhotos = false — без предела числа фото (открытие альбома); timeout — срок обхода.
func countQuietCap(root string, capPhotos bool, timeout time.Duration) agentplaces.QuietTally {
	// Подмены читаем до горутины: зависшая порция переживает предел, а тест восстановит их раньше.
	h := agentplaces.QuietHooks{Dev: placeDev, Remote: quietRemote, Pause: quietPause, Cloud: quietCloud, CapPhotos: capPhotos}
	return agentplaces.CountQuiet(root, timeout, h)
}
