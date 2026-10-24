// Правка текста на развороте (спека 006, design §3): выделение блока или подписи, сессия ввода,
// отложенная отправка набранного, правки стиля. Геометрию и перенос строк считает Go;
// ответ view во время ввода не трогает DOM редактора (он не привязан к view).
import { store, Api } from '../../base/core/store.svelte'
import { colorLive } from '../../base/color/colorlive.svelte'
import { styleSupport } from './fonts'
import { clampSize, isBlank, sessionId } from './textedit'
import { normDeg } from '../../base/geom/textbox'
import { clampPad, padOf, sampleCaption, startCaps } from '../captions/sheetcaps'
import { token, type Field } from '../captions/fields'
import { sheetIn, sheetsOf } from '../../base/geom/sheet'
import type { CaptionView, SheetCaps, SpreadView, TextStyle, TextView } from '../../base/core/types'

// Что выделено: свободный блок (id) или подпись к фото (id фото).
export interface TextSel {
  kind: 'text' | 'caption'
  spread: string
  id: string
}

// SetSheetCaps: структура вида (SheetCaps) уходит в Go как есть; тип биндинга — класс с методами.
const sheetCall = (spread: string, on: boolean, c: SheetCaps) =>
  Api.SetSheetCaps(spread, on, c as unknown as Parameters<typeof Api.SetSheetCaps>[2])

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
  // Отправки вне цепочки текста (имена в карточке, 087-9): «досыльщики» отложенного и летящие вызовы Go.
  private flushers = new Set<() => void>()
  private flying = new Set<Promise<unknown>>()

  get editing() {
    return this.target !== null
  }

  get spread(): SpreadView | undefined {
    const s = this.sel
    return s ? sheetIn(store.view, s.spread) : undefined
  }

  get text(): TextView | undefined {
    const s = this.sel
    return s?.kind === 'text' ? this.spread?.texts.find((t) => t.id === s.id) : undefined
  }

  get caption(): CaptionView | undefined {
    const s = this.sel
    return s?.kind === 'caption' ? this.spread?.captions.find((c) => c.photo === s.id) : undefined
  }

  // Общий стиль подписей текущего листа выделения (030); нет — у каждой подписи свой.
  get caps(): SheetCaps | undefined {
    return this.spread?.caps
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
    for (const f of [...this.flushers]) f()
    const wait = [this.chain, ...this.flying]
    await Promise.allSettled(wait)
  }

  // Другой отправитель (имена) регистрирует досылку отложенного: idle() зовёт её и ждёт track-нутые вызовы.
  // Возвращает отписку.
  onIdle(flush: () => void): () => void {
    this.flushers.add(flush)
    return () => this.flushers.delete(flush)
  }

  // Летящий вызов Go, который idle() обязан дождаться (смена альбома, сохранение, отмена).
  track<T>(p: Promise<T>): Promise<T> {
    this.flying.add(p)
    const drop = () => this.flying.delete(p)
    p.then(drop, drop)
    return p
  }

  // ---------- новые блоки ----------

  async addText() {
    const sp = store.spread
    if (!sp) return
    await this.finish()
    const session = this.newSession()
    if (!(await store.run(Api.AddText(sp.id, this.lastPage, session)))) return
    const t = sheetIn(store.view, sp.id)?.texts.at(-1)
    if (!t) return
    this.sel = { kind: 'text', spread: sp.id, id: t.id }
    store.selectedCell = null
    this.begin(true, session)
  }

  // Подпись к фото (кнопка «Подпись» в ленте EditRail): нет подписи — ставим заготовку, как у блока.
  async editCaption(photo: string) {
    const sp = sheetsOf(store.view).find((s) => s.photos.includes(photo))
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

  setStyle(patch: Partial<TextStyle>): Promise<unknown> | undefined {
    const st = this.style
    const s = this.sel
    if (!st || !s) return
    const next: TextStyle = { ...st, ...patch }
    if (patch.size !== undefined) next.size = clampSize(patch.size, st.size)
    const sup = styleSupport(next.font, store.fonts)
    next.bold = !!next.bold && sup.bold
    next.italic = !!next.italic && sup.italic
    if (s.kind === 'text') return store.run(Api.SetTextStyle(s.id, next))
    return this.putCaption(next, !!this.caption?.over, padOf(this.caption))
  }

  // Живой показ цвета из окна цвета (109): что поменяет setStyle — блок, подпись или (общий стиль листа, 030)
  // все подписи листа. null — снять. В Go не ходит, шага отмены нет.
  previewColor(hex: string | null) {
    const s = this.sel
    if (hex === null || !s) return colorLive.clear()
    if (s.kind === 'text') return colorLive.set([`text:${s.id}`], hex)
    const all = this.caps ? (sheetIn(store.view, s.spread)?.captions ?? []).map((c) => c.photo) : [s.id]
    colorLive.set(
      all.map((p) => `cap:${p}`),
      hex,
    )
  }

  // Подпись под фото / на фото.
  setOver(over: boolean) {
    const st = this.style
    if (!st || this.sel?.kind !== 'caption') return
    this.putCaption(st, over, padOf(this.caption))
  }

  // Отступ подписи от фото, мм (на фото — от нижнего края).
  setPad(v: number): Promise<unknown> | undefined {
    const st = this.style
    if (!st || this.sel?.kind !== 'caption') return
    return this.putCaption(st, !!this.caption?.over, clampPad(v, padOf(this.caption)))
  }

  // Оформление подписи: при общем стиле листа правится он (разом у всех подписей листа), иначе своя подпись.
  private putCaption(st: TextStyle, over: boolean, pad: number): Promise<unknown> | undefined {
    const s = this.sel
    const caps = this.caps
    if (!s || s.kind !== 'caption') return
    if (caps) return store.run(sheetCall(s.spread, true, { ...caps, style: st, over, pad }))
    return store.run(Api.SetCaptionStyle(s.id, st, over, pad))
  }

  // «Один стиль на лист» (спека 030): включить со стилем образца или снять.
  setSheet(spreadId: string, on: boolean) {
    const sp = sheetIn(store.view, spreadId)
    if (!sp) return
    if (!on) return sp.caps ? store.run(sheetCall(spreadId, false, sp.caps)) : undefined
    const sample = sampleCaption(sp, sp.id === this.sel?.spread ? this.caption : undefined)
    if (sample) return store.run(sheetCall(spreadId, true, startCaps(sample)))
  }

  // Шаблон подписи фото без своей и «ужимать» — поля стиля листа.
  setSheetPart(spreadId: string, patch: Partial<Pick<SheetCaps, 'text' | 'wrap'>>) {
    const caps = sheetIn(store.view, spreadId)?.caps
    if (caps) return store.run(sheetCall(spreadId, true, { ...caps, ...patch }))
  }

  // Поле в позицию курсора: в открытом редакторе — вставкой текста (курсор остаётся, набор уходит обычным путём),
  // иначе в конец шаблона выделенного блока или подписи.
  async insertField(f: Field) {
    const s = this.sel
    if (!s) return
    const t = token(f)
    if (this.target && !this.closing && document.execCommand('insertText', false, t)) return
    // Недосланный набор уйдёт первым: иначе SetText с шаблоном затёр бы только что набранное.
    await this.idle()
    const cur = this.text ?? this.caption
    if (!cur) return
    const text = (cur.src ?? cur.text) + t
    const session = this.newSession()
    await store.run(s.kind === 'text' ? Api.SetText(s.id, text, session) : Api.SetCaption(s.id, text, session))
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
