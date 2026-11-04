//go:build windows

package salbum

import (
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// TestDiskCloudStub (160-M1): заглушка OneDrive — файл с размером (fileSizeOn, isFileOn), а размеры фото не читаются
// без скачивания (probeDimsOn отказывает, поставщик не вызывается).
func TestDiskCloudStub(t *testing.T) {
	root := testkit.NewCloudRoot(t)
	data := []byte("12345")
	p := root.Stub(t, "a.jpg", data)
	t.Run("размер", func(t *testing.T) {
		if n, ok := fileSizeOn(osDisk{}, p); !ok || n != int64(len(data)) {
			t.Errorf("fileSizeOn = %d, %v", n, ok)
		}
		if !isFileOn(osDisk{}, p) {
			t.Error("isFileOn: заглушка не файл")
		}
	})
	t.Run("размеры без скачивания", func(t *testing.T) {
		if _, _, ok := probeDimsOn(osDisk{}, p); ok {
			t.Error("probeDimsOn прочитал заглушку")
		}
		if n := root.Fetches(); n != 0 {
			t.Errorf("скачано %d раз", n)
		}
	})
}
