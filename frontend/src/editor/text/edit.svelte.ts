// Правка текста на развороте (спека 006, design §3): выделение блока или подписи, сессия ввода,
// отложенная отправка набранного, правки стиля. Геометрию и перенос строк считает Go;
// ответ view во время ввода не трогает DOM редактора (он не привязан к view).
import { store, Api } from '../../base/core/store.svelte'
import { styleSupport } from './fonts'
import { clampSize, isBlank, sessionId } from './textedit'
import { normDeg } from '../../base/geom/textbox'
import type { CaptionView, SpreadView, TextStyle, TextView } from '../../base/core/types'

// Что выделено: свободный блок (id) или подпись к фото (id фото).
export interface TextSel {
  kind: 'text' | 'caption'
  spread: string
  id: string
}

// Пауза после последней клавиши, после которой набранное уходит в Go.
export const SEND_DELAY = 700

class Edit {
  sel = $state<TextSel | null>(null)
  // Идёт ввод в выделенное; target — то, куда уходит набранное (sel мог уже смениться).
  target = $state<TextSel | null>(null)
  // Esc нажат, ответ Go ещё не пришёл: редактор уже не принимает ввод и не держит фокус,
  // но остаётся на экране с набранным текстом (иначе мигнул бы старый, пока идёт сохранение).
  closing = $state(false)
  // Заготовку нового блока при входе выделяем целиком.
  selectAll = false
  // Страница под курсором на развороте (для кнопки «Текст»).
  lastPage: 0 | 1 = 0

  private session = ''
  private seq = 0
  private pending: string | null = null // набрано, но не отправлено
  private timer = 0
  private chain: Promise<unknown> = Promise.resolve()

  get editing() {
    return this.target !== null
  }

  get spread(): SpreadView | undefined {
    const s = this.sel
    return s ? store.view.spreads.find((x) => x.id === s.spread) : undefined
  }

  get text(): TextView | undefined {
    const s = this.sel
    return s?.kind === 'text' ? this.spread?.texts.find((t) => t.id === s.id) : undefined
  }

  get caption(): CaptionView | undefined {
    const s = this.sel
    return s?.kind === 'caption' ? this.spread?.captions.find((c) => c.photo === s.id) : undefined
  }

  get style(): TextStyle | undefined {
    return (this.text ?? this.caption)?.style
  }

  // ---------- выделение ----------

  // Выделение фото и текста взаимоисключающие.
  select(sel: TextSel) {
    if (this.target && (this.target.kind !== sel.kind || this.target.id !== sel.id)) void this.finish()
    this.sel = sel
    store.selectedCell = null
  }

  // Снять выделение (конец ввода, если шёл).
  clear() {
    void this.finish()
    this.sel = null
  }

  // Блок или подпись исчезли (undo, удаление, пустой текст на выходе): выделение и ввод больше не к чему.
  dropIfGone() {
    if (!this.sel || this.text || this.caption) return
    this.stopTimer()
    this.pending = null
    this.target = null
    this.closing = false
    this.sel = null
  }

  // ---------- ввод ----------

  private begin(selectAll: boolean, session?: string) {
    if (!this.sel) return
    this.session = session ?? this.newSession()
    this.selectAll = selectAll
    this.pending = null
    this.closing = false
    this.target = this.sel
  }

  private newSession() {
    return sessionId(Date.now(), ++this.seq)
  }

  // Двойной щелчок по блоку или подписи.
  startEdit() {
    if (!this.target) this.begin(false)
  }

  // Набранное: уходит через SEND_DELAY после последней клавиши и на выходе.
  input(text: string) {
    this.pending = text
    this.stopTimer()
    this.timer = window.setTimeout(() => this.flush(false), SEND_DELAY)
  }

  private stopTimer() {
    clearTimeout(this.timer)
    this.timer = 0
  }

  // Пустой текст Go считает «удалить блок»: во время ввода его не шлём (блок исчез бы под курсором),
  // только на выходе.
  private flush(final: boolean): Promise<unknown> {
    this.stopTimer()
    const t = this.target
    const text = this.pending
    if (!t || text === null || (!final && isBlank(text))) return this.chain
    this.pending = null
    const session = this.session
    const op = () => store.run(t.kind === 'text' ? Api.SetText(t.id, text, session) : Api.SetCaption(t.id, text, session))
    this.chain = this.chain.then(op, op)
    return this.chain
  }

  // Выход из ввода (Esc, Ctrl+Enter, клик мимо): редактор гаснет сразу, набранное уходит в Go,
  // после ответа редактор убирается.
  async finish() {
    const t = this.target
    if (!t) return
    this.closing = true
    await this.flush(true)
    if (this.target === t) {
      this.target = null
      this.closing = false
    }
  }

  // Все отправки текста завершены, включая ещё не ушедшее набранное (его досылаем сразу, не дожидаясь паузы):
  // Ctrl+Z после Esc, сохранение, экспорт и правки слоёв ждут этого, иначе сработают раньше записи.
  async idle(): Promise<void> {
    this.stopTimer()
    if (this.pending !== null) void this.flush(false)
    await this.chain.then(
      () => {},
      () => {},
    )
  }

  // ---------- новые блоки ----------

  async addText() {
    const sp = store.spread
    if (!sp) return
    await this.finish()
    const session = this.newSession()
    if (!(await store.run(Api.AddText(sp.id, this.lastPage, session)))) return
    const t = store.view.spreads.find((s) => s.id === sp.id)?.texts.at(-1)
    if (!t) return
    this.sel = { kind: 'text', spread: sp.id, id: t.id }
    store.selectedCell = null
    this.begin(true, session)
  }

  // Подпись к фото (кнопка «Подпись» в ленте EditRail): нет подписи — ставим заготовку, как у блока.
  async editCaption(photo: string) {
    const sp = store.view.spreads.find((s) => s.photos.includes(photo))
    if (!sp) return
    await this.finish()
    const session = this.newSession()
    const had = sp.captions.some((c) => c.photo === photo)
    if (!had && !(await store.run(Api.SetCaption(photo, 'Подпись', session)))) return
    this.sel = { kind: 'caption', spread: sp.id, id: photo }
    store.selectedCell = null
    this.begin(!had, session)
  }

  // ---------- правки выделенного ----------

  setStyle(patch: Partial<TextStyle>) {
    const st = this.style
    const s = this.sel
    if (!st || !s) return
    const next: TextStyle = { ...st, ...patch }
    if (patch.size !== undefined) next.size = clampSize(patch.size, st.size)
    const sup = styleSupport(next.font, store.fonts)
    next.bold = !!next.bold && sup.bold
    next.italic = !!next.italic && sup.italic
    if (s.kind === 'text') store.run(Api.SetTextStyle(s.id, next))
    else store.run(Api.SetCaptionStyle(s.id, next, !!this.caption?.over))
  }

  // Подпись под фото / на фото.
  setOver(over: boolean) {
    const st = this.style
    const s = this.sel
    if (!st || s?.kind !== 'caption') return
    store.run(Api.SetCaptionStyle(s.id, st, over))
  }

  // Delete на выделенном: набранное не отправляем, блок или подпись уходят.
  async removeSelected() {
    const s = this.sel
    if (!s) return
    this.stopTimer()
    this.pending = null
    this.target = null
    this.closing = false
    this.sel = null
    await this.chain
    await store.run(s.kind === 'text' ? Api.DeleteText(s.id) : Api.SetCaption(s.id, '', ''))
  }

  // Ширина и положение блока после переноса или ручки.
  setBox(id: string, x: number, y: number, w: number) {
    return store.run(Api.SetTextBox(id, x, y, w))
  }

  // Поворот вокруг центра (Go сам пересчитывает x, y); один шаг undo на правку.
  setAngle(id: string, deg: number) {
    return store.run(Api.SetTextAngle(id, normDeg(deg)))
  }

  // Угловая ручка: новый кегль, рамка растёт вместе с ним (один шаг undo).
  setScale(id: string, size: number, x: number, y: number) {
    return store.run(Api.SetTextScale(id, size, x, y))
  }

  // Ширина снова по тексту.
  setAuto(id: string) {
    return store.run(Api.SetTextAuto(id))
  }
}

export const edit = new Edit()
