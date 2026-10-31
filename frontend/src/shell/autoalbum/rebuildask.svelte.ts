// Вопрос «Пересобрать альбом?» (107-5): открыт, пока ask не null. Окно — RebuildAskDialog; «Пересобрать» зовёт ok.
// Устроен как вопрос «Вернуть авто?» (gridAct.autoAsk): безопасный ответ слева и в фокусе.
export interface RebuildAsk {
  lines: string[]
  ok: () => void
}

class RebuildAskState {
  ask = $state.raw<RebuildAsk | null>(null)

  open(lines: string[], ok: () => void): void {
    this.ask = { lines, ok }
  }

  confirm(): void {
    const a = this.ask
    this.ask = null
    a?.ok()
  }

  cancel(): void {
    this.ask = null
  }
}

export const rebuildAsk = new RebuildAskState()
