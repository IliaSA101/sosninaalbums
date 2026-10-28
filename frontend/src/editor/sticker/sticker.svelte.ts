// Стикеры на листе (048 Ф2): жесты одного стикера (перенос, угол, ⟳), вызовы Go и меню правого щелчка.
// Выделение живёт в placesel (stickers), предпросмотр жеста — placesel.liveStickers; здесь — только жесты и правки.
// Геометрию зажимает и пишет Go (SetStickerBox): на отпускании уходит одна правка. Группа едет через groupgesture.
import { Api, store } from '../../base/core/store.svelte'
import { errText } from '../../base/words/errtext'
import { axisLock } from '../../base/geom/gridedit'
import { bdAngle } from '../../base/geom/bdbox'
import { cursorDeg } from '../../base/geom/textbox'
import { placesel } from '../places/placesel.svelte'
import { drive, snapCtx } from '../places/placegesture'
import { FadeLive } from '../backdrop/fade.svelte'
import { PatSession } from '../pattern/pattern'
import { boxOf, centerOf, moveBox, resizeBox, sameBox, stickerById, type StBox, type StCorner } from './sticker'
import type { StickerView, View } from '../../base/core/types'

// Живой просмотр прозрачности: пока тянут ползунок, стикер рисуется по этому значению (ключ — ID стикера).
export const stickerFadeLive = new FadeLive()

class StickerActions {
  // Сессия ползунков карточки: фокус → стрелки сливаются в один шаг отмены.
  readonly session = new PatSession()

  // Рамка стикера со всеми правками жеста (предпросмотр) — её рисуют слой и рамка выделения.
  boxOf(st: StickerView): StBox {
    const l = placesel.liveStickers?.get(st.id)
    return l ? { x: l.x, y: l.y, w: l.w, h: l.h, rot: l.rot ?? 0 } : boxOf(st)
  }

  // Стикер текущего листа по ID.
  find(id: string): StickerView | undefined {
    return stickerById(store.spread, id)
  }

  private show(id: string, b: StBox | null) {
    placesel.liveStickers = b ? new Map([[id, b]]) : null
  }

  // Итог жеста — одна правка рамки; холостой (рамка та же) Go не получает.
  private commit(spread: string, st: StickerView, b: StBox) {
    if (sameBox(b, boxOf(st))) return Promise.resolve(false)
    return store.run(Api.SetStickerBox(spread, st.id, b.x, b.y, b.w, b.h, b.rot, ''))
  }

  // Перенос: центр зажат в лист, края прилипают к линиям и местам (Alt — без привязки, Shift держит ось).
  // Щелчок без движения — onClick (у группы: оставить один этот стикер).
  move(e: PointerEvent, root: HTMLElement, st: StickerView, onClick?: () => void) {
    const s = store.spread
    if (!s) return
    const base = boxOf(st)
    drive(
      e,
      root,
      (dx, dy, ev) => {
        const a = axisLock(dx, dy, ev.shiftKey)
        const m = moveBox(base, a.dx, a.dy, snapCtx(ev, root, s, new Set()), a.axis, s.w, s.h)
        placesel.guides = { x: m.guideX, y: m.guideY }
        return m.box
      },
      (b) => this.show(st.id, b),
      (b) => this.commit(s.id, st, b),
      onClick,
    )
  }

  // Угловая ручка: пропорция держится всегда, у растяжимых (плашка, волна-лента) — свободно.
  resize(e: PointerEvent, root: HTMLElement, st: StickerView, corner: StCorner) {
    const s = store.spread
    if (!s) return
    const base = boxOf(st)
    drive(
      e,
      root,
      (dx, dy) => resizeBox(base, corner, dx, dy, !!st.stretch, s.w, s.h),
      (b) => this.show(st.id, b),
      (b) => this.commit(s.id, st, b),
    )
  }

  // ⟳: угол следит за курсором вокруг центра (Shift — 15°, Alt — без прилипания); на отпускании один SetStickerBox.
  // Результат шага — объект: угол 0° ложен, а drive по ложному считает, что сдвига не было.
  rotate(e: PointerEvent, root: HTMLElement, st: StickerView) {
    const s = store.spread
    if (!s) return
    const r = root.getBoundingClientRect()
    const k = s.w / r.width
    const base = boxOf(st)
    const c = centerOf(base)
    const p0 = { x: (e.clientX - r.left) * k, y: (e.clientY - r.top) * k }
    const grab = cursorDeg(c, p0)
    drive(
      e,
      root,
      (dx, dy, ev) => ({ b: { ...base, rot: bdAngle(c, { x: p0.x + dx, y: p0.y + dy }, grab, base.rot, ev.shiftKey, ev.altKey) } }),
      (a) => this.show(st.id, a ? a.b : null),
      (a) => this.commit(s.id, st, a.b),
    )
  }

  // Двойной щелчок по ⟳ — угол 0°.
  zero(st: StickerView) {
    const s = store.spread
    if (!s || !st.rot) return
    return this.commit(s.id, st, { ...boxOf(st), rot: 0 })
  }

  // ---------- правки через Go ----------

  // «Стикер» из окна: центр страницы, ¼ ширины; новый выделяется. page — страница под курсором.
  add(spread: string, page: 0 | 1, item: string): Promise<boolean> {
    return this.put(() => Api.AddSticker(spread, page, item))
  }

  // «QR-код по ссылке…»: Go кладёт код в «Мои» и ставит стикером на страницу одним шагом отмены. Ошибка — тост.
  addQR(spread: string, page: 0 | 1, link: string): Promise<boolean> {
    return this.put(() => Api.AddQRSticker(spread, page, link))
  }

  // Ответ Go с новым стикером: вид принимается, стикер выделяется.
  private async put(call: () => Promise<{ view: unknown; id?: string }>): Promise<boolean> {
    try {
      const r = await call()
      if (!(await store.take(r.view as View))) return false
      if (r.id) placesel.pickSticker(r.id, false)
      return true
    } catch (e) {
      store.notify(errText(e), 'error')
      return false
    }
  }

  // «Заменить…»: центр, ширина и поворот остаются.
  replace(spread: string, id: string, item: string) {
    return store.run(Api.ReplaceSticker(spread, id, item))
  }

  flip(axis: 'h' | 'v') {
    const s = store.spread
    const st = placesel.soloSticker
    if (s && st) return store.run(Api.FlipSticker(s.id, st.id, axis))
  }

  setInks(st: StickerView, inks: string[]) {
    const s = store.spread
    if (s) return store.run(Api.SetStickerInks(s.id, st.id, inks, ''))
  }

  // Ползунок прозрачности: один вызов на отпускании, стрелки в одном фокусе — один шаг отмены (merge).
  setFade(st: StickerView, fade: number, merge: string) {
    const s = store.spread
    if (!s) return Promise.resolve(false)
    return stickerFadeLive.commit(() => store.run(Api.SetStickerFade(s.id, st.id, fade, merge)))
  }

  // «Заменить…»: открыть окно «Стикеры» в режиме замены этого стикера.
  // Окно подключает вызывающий (decor.show) — здесь только ID листа и стикера.
  target(): { spread: string; id: string } | null {
    const s = store.spread
    const st = placesel.soloSticker
    return s && st ? { spread: s.id, id: st.id } : null
  }
}

export const sticker = new StickerActions()

// Страница под курсором: стикер встаёт на ту же, что и «+ Место».
export const currentPage = (): 0 | 1 => (placesel.page === 1 ? 1 : 0)
