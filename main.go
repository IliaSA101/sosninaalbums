package main

import (
	"embed"
	"log"

	"github.com/IliaSA101/sosninaalbums/internal/app"
)

// Встраивание — только здесь: embed не ходит вверх по папкам, а фронт и wails.json лежат в корне.
//
//go:embed all:frontend/dist
var assets embed.FS

//go:embed wails.json
var wailsJSON []byte

// Значок окна Linux (tar.gz без .desktop): тот же appicon, что у exe.
//
//go:embed build/appicon.png
var appIcon []byte

func main() {
	if err := app.Run(assets, wailsJSON, appIcon); err != nil {
		log.Println("Ошибка:", err)
	}
}
