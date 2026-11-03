package osx

import (
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ProbeTimeout — сколько ждать ответа корня тома. Отключённый сетевой диск (SMB) отвечает
// только по таймауту редиректора, десятки секунд; столько окно и Undo ждать не должны.
const ProbeTimeout = 1500 * time.Millisecond

// Сколько помнить ответ тома: доступный проверяем чаще, мёртвый — реже (каждая его проба —
// ProbeTimeout ожидания).
const (
	liveTTL = 5 * time.Second
	deadTTL = 15 * time.Second
)

// Volumes — проверка доступности томов с кэшем и одной пробой на том (single-flight).
// Отменить висящий os.Stat нельзя: проба, не уложившаяся в таймаут, доживает в своей
// горутине и, когда закончится, обновит кэш и позовёт обработчики OnBack.
type Volumes struct {
	stat    func(string) (fs.FileInfo, error)
	now     func() time.Time
	timeout time.Duration
	// volOf — том пути ("" — без тома); подменяется в тестах: буквы дисков есть только в Windows.
	volOf func(string) string

	mu     sync.Mutex
	vols   map[string]*volState
	backs  map[int]func(vol string) // обработчики «том ответил после мёртвого»
	nextID int
}

type volState struct {
	ok      bool
	until   time.Time     // ответ годен до (по now)
	probe   chan struct{} // идущая проба; nil — пробы нет
	started time.Time     // начало идущей пробы (настоящие часы: по ним ждём)
}

// NewVolumes — проверка с подменённым stat корня (тесты) и таймаутом пробы.
func NewVolumes(stat func(string) (fs.FileInfo, error), timeout time.Duration) *Volumes {
	return &Volumes{stat: stat, now: time.Now, timeout: timeout, volOf: VolumeKey,
		vols: map[string]*volState{}, backs: map[int]func(string){}}
}

// std — проверка процесса; qaStat — шов задержки пробы набора QA (volume_qa.go), в выпуске прямой os.Stat.
var std = NewVolumes(qaStat(os.Stat), ProbeTimeout)

// VolumeKey — том пути в верхнем регистре ("E:"); "" — у пути без тома (Unix) и у относительного.
func VolumeKey(path string) string { return strings.ToUpper(filepath.VolumeName(path)) }

// OnVolumeBack — обработчик «том ответил после того, как считался мёртвым»; off снимает только его.
func OnVolumeBack(fn func(vol string)) (off func()) { return std.OnBack(fn) }

// KickVolume — проба тома пути без ожидания (см. Volumes.Kick).
func KickVolume(path string) { std.Kick(path) }

// Reachable — отвечает ли том пути: перед обращением к файлу, который может лежать на
// отключённом сетевом диске. Пути без тома (Unix) — true. Ждёт не дольше ProbeTimeout.
func Reachable(path string) bool { return std.Reachable(path) }

// Reachable — см. пакетную Reachable.
func (v *Volumes) Reachable(path string) bool {
	vol := v.volOf(path)
	if vol == "" {
		return true
	}
	v.mu.Lock()
	s, ch := v.startLocked(vol)
	if ch == nil {
		ok := s.ok // ответ ещё годен
		v.mu.Unlock()
		return ok
	}
	wait := v.timeout - time.Since(s.started)
	v.mu.Unlock()

	t := time.NewTimer(max(wait, 0))
	defer t.Stop()
	select {
	case <-ch:
	case <-t.C:
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.now().Before(s.until) {
		return s.ok // проба ответила (наша или та, что закончилась чуть раньше)
	}
	// Таймаут: том считаем мёртвым на deadTTL, проба работает дальше.
	s.ok, s.until = false, v.now().Add(deadTTL)
	return false
}

// startLocked — состояние тома vol (под mu): ответ годен — ch == nil; иначе идущая проба
// (при надобности запускает новую). Общее для Reachable и Kick.
func (v *Volumes) startLocked(vol string) (s *volState, ch chan struct{}) {
	s = v.vols[vol]
	if s == nil {
		s = &volState{}
		v.vols[vol] = s
	}
	if v.now().Before(s.until) {
		return s, nil
	}
	if s.probe == nil {
		s.probe, s.started = make(chan struct{}), time.Now()
		go v.run(vol, s, s.probe)
	}
	return s, s.probe
}

// Kick запускает пробу тома пути, если ответ устарел, и не ждёт: ответ дойдёт до обработчиков
// OnBack. Нужен сверке папок: выдернутый и вставленный диск иначе не заметить до первого Reachable.
func (v *Volumes) Kick(path string) {
	vol := v.volOf(path)
	if vol == "" {
		return
	}
	v.mu.Lock()
	v.startLocked(vol)
	v.mu.Unlock()
}

// OnBack — обработчик «том ответил, хотя считался мёртвым» (проба не уложилась в таймаут или
// кончился срок мёртвого). Первый ответ нового тома и ответ живого — не событие. Обработчик
// зовётся до пробуждения ждущих Reachable той же пробы и должен быть коротким; Reachable из
// него не ждёт: кэш уже «жив». off снимает только свой обработчик.
func (v *Volumes) OnBack(fn func(vol string)) (off func()) {
	v.mu.Lock()
	defer v.mu.Unlock()
	id := v.nextID
	v.nextID++
	v.backs[id] = fn
	return func() {
		v.mu.Lock()
		defer v.mu.Unlock()
		delete(v.backs, id)
	}
}

// run — сама проба: os.Stat корня тома в своей горутине.
func (v *Volumes) run(vol string, s *volState, ch chan struct{}) {
	_, err := v.stat(vol + string(filepath.Separator))
	v.mu.Lock()
	wasDead := !s.ok && !s.until.IsZero() // до записи нового ответа
	ttl := deadTTL
	if err == nil {
		ttl = liveTTL
	}
	s.ok, s.until = err == nil, v.now().Add(ttl)
	if s.probe == ch {
		s.probe = nil
	}
	var fns []func(string)
	if err == nil && wasDead {
		fns = make([]func(string), 0, len(v.backs))
		for _, fn := range v.backs {
			fns = append(fns, fn)
		}
	}
	v.mu.Unlock()
	// До close(ch): ждущие Reachable этой пробы (пересчёт по фокусу) проснутся уже после снимка
	// обработчика, и их итог не обгонит снимок (157-7).
	for _, fn := range fns {
		callBack(fn, vol)
	}
	close(ch)
}

// callBack зовёт обработчик под recover: паника чужого кода не должна оставить ждущих без close(ch).
func callBack(fn func(string), vol string) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("проба тома: обработчик: %v", r)
		}
	}()
	fn(vol)
}
