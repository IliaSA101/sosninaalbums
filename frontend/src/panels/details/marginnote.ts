// Предупреждение о полях у линии реза (0.23-q7 а): текст считает Go (App.MarginNote по черновику формы),
// здесь — вызов с отбрасыванием устаревших ответов. Пустая строка — предупреждать не о чем.
export type MarginNoteCall = (margin: number, bleed: number) => Promise<string>

export interface MarginNoteAsker {
  // Спросить по текущему черновику; нечисло — строка гаснет без вызова Go.
  ask(margin: unknown, bleed: unknown): void
}

// Гонка: ответы приходят не по порядку — побеждает последний вызов (ask или сброс нечислом).
export function marginNoteAsker(call: MarginNoteCall, set: (text: string) => void): MarginNoteAsker {
  let seq = 0
  return {
    ask(margin, bleed) {
      const my = ++seq
      const m = Number(margin)
      const b = Number(bleed ?? 0)
      if (margin === '' || margin === null || !Number.isFinite(m) || !Number.isFinite(b)) return set('')
      call(m, b).then(
        (t) => my === seq && set(t ?? ''),
        () => my === seq && set(''), // сбой Go — форме не мешает, просто без предупреждения
      )
    },
  }
}
