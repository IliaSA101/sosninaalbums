package osx

import (
	"os"
	"path/filepath"
)

// WebViewDir — папка данных WebView2: %LOCALAPPDATA%\SosninaAlbums\WebView2. Без неё
// WebView2 кладёт данные в %AppData%\<имя exe>, а exe называется по версии, и каждая
// версия теряла бы localStorage. Local, а не Roaming: кэш большой. "" — пусть решает Wails.
func WebViewDir() string {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		base, _ = os.UserCacheDir()
	}
	if base == "" {
		return ""
	}
	return filepath.Join(base, "SosninaAlbums", "WebView2")
}
