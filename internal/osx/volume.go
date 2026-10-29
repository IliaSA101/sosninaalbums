package osx

import (
	"io/fs"
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
// горутине и, когда закончится, обновит кэш.
type Volumes struct {
	stat    func(string) (fs.FileInfo, error)
	now     func() time.Time
	timeout time.Duration

	mu   sync.Mutex
	vols map[string]*volState
}

type volState struct {
	ok      bool
	until   time.Time     // ответ годен до (по now)
	probe   chan struct{} // идущая проба; nil — пробы нет
	started time.Time     // начало идущей пробы (настоящие часы: по ним ждём)
}

// NewVolumes — проверка с подменённым stat корня (тесты) и таймаутом пробы.
func NewVolumes(stat func(string) (fs.FileInfo, error), timeout time.Duration) *Volumes {
	return &Volumes{stat: stat, now: time.Now, timeout: timeout, vols: map[string]*volState{}}
}

var std = NewVolumes(os.Stat, ProbeTimeout)

// Reachable — отвечает ли том пути: перед обращением к файлу, который может лежать на
// отключённом сетевом диске. Пути без тома (Unix) — true. Ждёт не дольше ProbeTimeout.
func Reachable(path string) bool { return std.Reachable(path) }

// Reachable — см. пакетную Reachable.
func (v *Volumes) Reachable(path string) bool {
	vol := strings.ToUpper(filepath.VolumeName(path))
	if vol == "" {
		return true
	}
	v.mu.Lock()
	s := v.vols[vol]
	if s == nil {
		s = &volState{}
		v.vols[vol] = s
	}
	if v.now().Before(s.until) {
		ok := s.ok
		v.mu.Unlock()
		return ok
	}
	if s.probe == nil {
		s.probe, s.started = make(chan struct{}), time.Now()
		go v.run(vol, s, s.probe)
	}
	ch, wait := s.probe, v.timeout-time.Since(s.started)
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

// run — сама проба: os.Stat корня тома в своей горутине.
func (v *Volumes) run(vol string, s *volState, ch chan struct{}) {
	_, err := v.stat(vol + string(filepath.Separator))
	v.mu.Lock()
	defer v.mu.Unlock()
	ttl := deadTTL
	if err == nil {
		ttl = liveTTL
	}
	s.ok, s.until = err == nil, v.now().Add(ttl)
	if s.probe == ch {
		s.probe = nil
	}
	close(ch)
}
