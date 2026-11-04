//go:build windows

package app

import (
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// TestCloudStubCountsAsFile (160-M1): облачная заглушка OneDrive — файл на месте, а не «пропавший» и не «папка или
// ссылка» при замене альбома (единый предикат imgfile.RegularOrCloud). Блоки независимы.
func TestCloudStubCountsAsFile(t *testing.T) {
	root := testkit.NewCloudRoot(t)
	p := root.Stub(t, "a.salbum", []byte("{}"))
	t.Run("fileExists", func(t *testing.T) {
		if !fileExists(p) {
			t.Error("заглушка считается пропавшей")
		}
	})
	t.Run("замена альбома", func(t *testing.T) {
		q, err := replaceAlbumQuestion(p)
		if err != nil || q == "" {
			t.Errorf("вопрос %q, ошибка %v: заглушка должна спрашивать о замене", q, err)
		}
	})
	t.Run("без скачивания", func(t *testing.T) {
		if n := root.Fetches(); n != 0 {
			t.Errorf("проверки скачали заглушку: %d", n)
		}
	})
}
