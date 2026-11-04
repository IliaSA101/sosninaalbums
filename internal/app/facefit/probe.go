package facefit

import (
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/geom"
)

// Доступ тестов App к памяти сессии: сами карты за замком, а сценарии «после перезапуска»,
// «норма листа после образца» и «кэш наполнился» проверяются из корня, где живёт настоящий App.

// ForgetFitted и ForgetNorm опустошают память сессии — как после перезапуска программы.
func (f *Fitter) ForgetFitted() { f.mu.Lock(); f.fitted = map[string]model.Frame{}; f.mu.Unlock() }
func (f *Fitter) ForgetNorm()   { f.mu.Lock(); f.norm = map[string]geom.FaceNorm{}; f.mu.Unlock() }

// SavedNorm — норма листа из памяти (ok = false — «Остальные — как это лицо» не было).
func (f *Fitter) SavedNorm(spreadID string) (n geom.FaceNorm, ok bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	n, ok = f.norm[spreadID]
	return n, ok
}

// FittedLen — сколько кадров запомнил подгон; CacheLen — записей в кэше точек; Busy — поиск идёт.
func (f *Fitter) FittedLen() int { f.mu.Lock(); defer f.mu.Unlock(); return len(f.fitted) }
func (f *Fitter) CacheLen() int  { return f.cache.size() }
func (f *Fitter) Busy() bool     { return f.busy.Load() }
