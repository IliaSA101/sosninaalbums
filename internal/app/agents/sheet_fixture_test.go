package agents

import "context"

// Подставные замыкания ступени 3б (151): лист здесь, правящие — fake34 (edit_run34_test.go).

// install34 заполняет семь полей Host ступени 3б.
func (e *editFake) install34(h *Host) {
	h.Sheet = func(ctx context.Context, in SheetReq) ([]byte, []string, error) {
		e.nSheet.Add(1)
		e.mu.Lock()
		e.lastSheet = in
		e.mu.Unlock()
		e.mu.Lock()
		fn := e.sheet
		e.mu.Unlock()
		if fn != nil {
			return fn(ctx, in)
		}
		b, err := testJPEG(in.Side, in.Side)
		states := make([]string, len(in.IDs))
		for i := range states {
			states[i] = CellOK
		}
		return b, states, err
	}
	h.Families = func(ids []string) []string {
		e.mu.Lock()
		fn := e.fams
		e.mu.Unlock()
		if fn != nil {
			return fn(ids)
		}
		return make([]string, len(ids))
	}
	e.fake34.install(e, h)
}

func (e *editFake) gotSheet() SheetReq { e.mu.Lock(); defer e.mu.Unlock(); return e.lastSheet }

// setSheet подменяет Host.Sheet на ходу: служба зовёт его из своих горутин.
func (e *editFake) setSheet(fn func(context.Context, SheetReq) ([]byte, []string, error)) {
	e.mu.Lock()
	e.sheet = fn
	e.mu.Unlock()
}
