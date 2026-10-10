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

func main() {
	if err := app.Run(assets, wailsJSON); err != nil {
		log.Println("Ошибка:", err)
	}
}
