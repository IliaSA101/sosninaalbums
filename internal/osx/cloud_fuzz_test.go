package osx

import (
	"path/filepath"
	"testing"
)

// FuzzCloudParse: разборщики настроек клиентов на произвольных байтах не паникуют, а корни после очистки всегда абсолютные,
// чистые и без управляющих знаков (чужой файл настроек — вход недоверенный).
func FuzzCloudParse(f *testing.F) {
	f.Add([]byte(`{"personal":{"path":"/home/u/Dropbox"},"business":{"path":"/home/u/Dropbox (Team)"}}`))
	f.Add([]byte("proxy=\"no\"\ndir=\"/home/u/Yandex.Disk\"\n"))
	f.Add([]byte(`{"personal":{"path":5}}`))
	f.Add([]byte("dir=\x00\n"))
	f.Fuzz(func(t *testing.T, data []byte) {
		raw := append(parseDropboxInfo(data), parseYandexConfig(data)...)
		for _, p := range cleanCloudRoots(raw, allowAll) {
			if !filepath.IsAbs(p) || filepath.Clean(p) != p || len(p) > maxCloudPath {
				t.Fatalf("негодный корень %q", p)
			}
			for _, r := range p {
				if r < 0x20 || r == 0x7f {
					t.Fatalf("управляющий знак в корне %q", p)
				}
			}
		}
	})
}
