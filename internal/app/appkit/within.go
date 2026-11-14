package appkit

import "time"

// Within — f с пределом d: не уложилась или упала с паникой — ok = false. Зависшая f доживает в
// своей горутине (отменить системный вызов нельзя, как у osx.Volumes).
func Within[T any](d time.Duration, f func() T) (v T, ok bool) {
	type result struct {
		v  T
		ok bool
	}
	ch := make(chan result, 1)
	go func() {
		var r result
		defer func() {
			_ = recover() // чужой диск не должен валить программу
			ch <- r
		}()
		r.v = f()
		r.ok = true
	}()
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case r := <-ch:
		return r.v, r.ok
	case <-t.C:
		return v, false
	}
}
