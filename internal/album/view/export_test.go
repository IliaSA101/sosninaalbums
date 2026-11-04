package view

import "github.com/IliaSA101/sosninaalbums/internal/album/model"

// Доступ внешних тестов (view_test) к крючку layWorker: тест подменяет счёт разворота,
// чтобы имитировать панику воркера; в рабочем коде крючок не виден.

// LayWorker — текущий счёт разворота в воркере.
func LayWorker() func(*model.Project, *model.Spread) Laid { return layWorker }

// SetLayWorker подменяет счёт разворота в воркере.
func SetLayWorker(f func(*model.Project, *model.Spread) Laid) { layWorker = f }
