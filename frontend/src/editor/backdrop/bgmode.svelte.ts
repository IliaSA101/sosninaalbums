// Режим фона (спека 009, design-backdrop §6): окно, не проект — в undo и файл не попадает.
// Решения — backdrop/bgmode.ts, геометрия жестов — geom/bdbox.ts; рамку и ручки рисует BackdropEditor.
import { Api, store } from '../../base/core/store.svelte'
import { edit } from '../text/edit.svelte'
import { placesel } from '../places/placesel.svelte'
import { gallery } from '../../panels/gallery/gallery.svelte'
import { backdropOf } from './backdrop'
import { bdSession, keepMode, type BgCur, type BgLive } from './bgmode'
import { nudgeBox } from '../../base/geom/bdbox'
import type { BackdropScope, BackdropView } from '../../base/core/types'

class BgMode {
  cur = $state<BgCur | null>(null)
  // Предпросмотр жеста (рамка и кадр до отпускания); его же рисует BackdropLayer.
  live = $state<BgLive | null>(null)
  // Направляющие прилипания при переносе, мм.
  guides = $state<{ x: number | null; y: number | null }>({ x: null, y: null })
  // Серия стрелок — один шаг undo; любой другой жест начинает новую.
  private session = ''
  private chain: Promise<unknown> = Promise.resolve()

  constructor() {
    // Выход, когда разворот сменился или фона этой области больше нет (Undo, «Убрать», чужая правка).
    $effect.root(() => {
      $effect(() => {
        const selected =
          !!store.selectedCell || !!edit.sel || placesel.refs.length > 0 || placesel.texts.length > 0 || placesel.stickers.length > 0
        if (this.cur && !keepMode(this.cur, store.spread, selected)) this.exit()
      })
    })
  }

  // Фон, который правят, из текущего View.
  get view(): BackdropView | undefined {
    const c = this.cur
    return c && store.spread?.id === c.spread ? backdropOf(store.spread, c.scope) : undefined
  }

  // Вход: кнопка «Править» в карточке и сразу после постановки фото на фон. Снимает выделение,
  // мини-панель и галерею.
  enter(spread: string, scope: BackdropScope) {
    placesel.clear()
    edit.clear()
    store.selectedCell = null
    gallery.close()
    this.cur = { spread, scope }
    this.reset()
  }

  exit() {
    this.cur = null
    this.reset()
  }

  private reset() {
    this.live = null
    this.guides = { x: null, y: null }
    this.session = ''
  }

  // Правка рамки одним шагом undo (жест); merge — только у серии стрелок.
  async setBox(box: { x: number; y: number; w: number; h: number; rot: number }, merge = '') {
    const c = this.cur
    if (!c) return
    if (!merge) this.session = ''
    await store.run(Api.SetBackdropBox(c.spread, c.scope, box.x, box.y, box.w, box.h, box.rot, merge))
  }

  async setFrame(f: { s: number; x: number; y: number }) {
    const c = this.cur
    if (!c) return
    this.session = ''
    await store.run(Api.SetBackdropFrame(c.spread, c.scope, f.s, f.x, f.y))
  }

  // Наклон фото в рамке (045): поле и ползунок «Наклон, °»; session (057) — сессия строки «Наклон», в Go ключ merge:
  // протяжки одной сессии — один шаг undo.
  async setAngle(tilt: number, session: string) {
    const c = this.cur
    if (!c) return
    this.session = ''
    await store.run(Api.SetBackdropAngle(c.spread, c.scope, tilt, session))
  }

  // Стрелки: очередь, чтобы удержанная клавиша считала от актуального View.
  nudge(key: string, shift: boolean) {
    if (!this.cur) return
    if (!this.session) this.session = bdSession()
    const merge = this.session
    this.chain = this.chain.then(async () => {
      const b = this.view
      const t = b && nudgeBox(b.box, key, shift)
      if (t) await this.setBox(t, merge)
    })
  }

  // Del / «Убрать»: фото фона снимается; режим закроет keepMode, когда View придёт без фона
  // (при ошибке Go режим остаётся).
  async remove() {
    const c = this.cur
    if (!c) return
    await store.run(Api.ClearBackdrop(c.spread, c.scope))
  }
}

export const bgmode = new BgMode()
