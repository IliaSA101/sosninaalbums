// Постановка из окна «Фоны» (048, 067, 077, 079, 098, 100-8): щелчок по плитке, вопросы перед постановкой, «На все» в тосте.
// Состояние окна живёт в DecorUi (host); здесь только ход постановки и серия «тот же фон на соседний лист».
import { Api, store } from '../../base/core/store.svelte'
import { SvelteSet } from 'svelte/reactivity'
import { errText } from '../../base/words/errtext'
import type { BackdropScope, GridPlan, GridTarget } from '../../base/core/types'
import { isSheet, sheetIn, unitOf } from '../../base/geom/sheet'
import { applyBackground } from '../../editor/backdrop/backdropact'
import { currentPage, sticker } from '../../editor/sticker/sticker.svelte'
import { standingId, type DecorItem, type TplResult } from './decor'
import {
  bgAskText,
  bgDoneText,
  bgMissing,
  bgOfferText,
  bgPickShown,
  bgStreak,
  bgTargets,
  isMany,
  type BgPick,
  type BgPlanInfo,
  type BgPut,
  type BgResult,
} from './bgtargets'
import { putQuestion, putToast } from './spreadtpl'
import { ownAskText, ownBgOf, ownDoneText } from '../albumbg/albumown'
import { FRAME_NO_PLACES } from './frames'
import { frameApi } from './frameapi'
import type { DecorAsk, DecorCur } from './decor.svelte'

// Что постановке нужно от окна (структурный тип: DecorUi передаёт себя, цикла значений нет).
export interface DecorPutHost {
  readonly cur: DecorCur | null
  ask: DecorAsk | null
  busy: boolean
  bgPick: BgPick
  numsFocus: number
  readonly albumPages: boolean
  readonly bgItems: DecorItem[]
  close(): void
  show(kind: DecorCur['kind'], spread: string, scope: BackdropScope, replace?: string): void
  bgPlan(): { pick: BgPick; targets: GridTarget[]; error: string | null }
  albumNow(): boolean
  frameIds(): string[]
  allIds(): string[]
}

export class DecorPut {
  // 100-8: серия «тот же фон на соседний лист»; фоны, о которых уже спрашивали, — до смены альбома (store.openSeq).
  private lastPut: BgPut | null = null
  private offered = new SvelteSet<string>()
  private seriesSeq = 0

  constructor(private h: DecorPutHost) {}

  // Щелчок по плитке: окно закрывается сразу (как галерея ⊞), правку ставит Go одним шагом. Фон — в область;
  // стикер — в центр страницы под курсором либо вместо выделенного («Заменить…»).
  async pick(item: DecorItem) {
    const c = this.h.cur
    if (!c) return
    if (c.kind === 'spread') return this.pickSpread(item)
    if (c.kind === 'bg') return this.pickBg(c, item)
    if (c.kind === 'frame') return this.pickFrame(c.spread, item)
    this.h.close()
    if (c.replace) await sticker.replace(c.spread, c.replace, item.id)
    else await sticker.add(c.spread, currentPage(), item.id)
  }

  // Рамочка (079): на выделенные места или на все места листа; окно закрывается сразу, правку ставит Go одним шагом
  // (та же рамочка уже везде — холостая, без шага). Нет мест — подсказка, окно остаётся.
  private async pickFrame(spread: string, item: DecorItem) {
    const ids = this.h.frameIds()
    if (ids.length === 0) {
      store.notify(FRAME_NO_PLACES)
      return
    }
    this.h.close()
    await store.run(frameApi.apply(spread, ids, item.id))
  }

  // Фон (077): «этот лист» — прежний путь (окно закрыто, своё — сразу режим фона); набор — сухой прогон, вопрос,
  // постановка одним вызовом. Неверные номера ничего не ставят: фокус в поле, ошибка под ним.
  private async pickBg(c: DecorCur, item: DecorItem) {
    if (this.h.albumNow()) {
      // 098: одна правка Go — один шаг отмены; тот же фон повторно холостой (rev не меняется)
      const pages = this.h.albumPages && !isSheet(store.view.settings)
      // 115: у разворотов свой фон, и фон альбома на них не виден — сначала вопрос в этом же окне (тот же фон тоже спрашивает)
      const own = ownBgOf(store.view)
      if (own.nums.length > 0) {
        this.h.ask = {
          kind: 'albumOwn',
          id: item.id,
          pages,
          own: ownAskText('bg', own, unitOf(store.view.settings)),
          count: own.nums.length,
        }
        return
      }
      this.h.close()
      await store.run(Api.SetAlbumBg(item.id, pages))
      return
    }
    const { pick, targets, error } = this.h.bgPlan()
    if (error) {
      this.h.numsFocus++
      return
    }
    if (!isMany(pick, targets, c.spread)) {
      this.h.close()
      await applyBackground(c.spread, c.scope, item.id)
      this.offerAll(c.spread, c.scope, item.id)
      return
    }
    this.lastPut = null
    if (this.h.busy) return
    this.h.busy = true
    try {
      const plan = (await Api.PlanBackgroundTo(targets, item.id)) as unknown as BgPlanInfo
      if (this.h.cur !== c) return // окно закрыли, пока считался вопрос
      const text = bgAskText(plan, unitOf(store.view.settings))
      if (text) this.h.ask = { kind: 'bgMany', id: item.id, text, targets }
      else await this.putMany(targets, item.id)
    } catch (e) {
      store.notify(errText(e), 'error')
    } finally {
      this.h.busy = false
    }
  }

  // 100-8: второй подряд тот же фон на соседний лист — тост «На все развороты?» (один раз на фон за открытие альбома).
  // Только если фон встал: холостая или отменённая постановка серии не продолжает.
  private offerAll(spread: string, scope: BackdropScope, id: string) {
    if (this.seriesSeq !== store.openSeq) {
      this.seriesSeq = store.openSeq
      this.lastPut = null
      this.offered.clear()
    }
    const sp = sheetIn(store.view, spread)
    const index = store.view.spreads.findIndex((s) => s.id === spread)
    if (!bgPickShown(sp, store.view.spreads.length) || index < 0 || standingId(sp, scope, this.h.bgItems) !== id) {
      this.lastPut = null
      return
    }
    const cur: BgPut = { item: id, scope, index }
    // Прошлый лист серии должен всё ещё стоять с этим фоном: отмену между постановками серия не переживает.
    const prev = this.lastPut ? store.view.spreads[this.lastPut.index] : undefined
    if (this.lastPut && (!prev || standingId(prev, this.lastPut.scope, this.h.bgItems) !== this.lastPut.item)) this.lastPut = null
    const miss = bgMissing(store.view.spreads, scope, id, this.h.bgItems)
    if (miss > 0 && bgStreak(this.lastPut, cur, this.offered)) {
      this.offered.add(id)
      const seq = store.openSeq
      store.notify(bgOfferText(miss, unitOf(store.view.settings)), 'info', {
        label: 'На все',
        run: () => void this.putAll(id, scope, spread, seq),
      })
      // Любая правка и Ctrl+Z гасят тост (как «Вернуть»): серия не переживает отмену.
      if (store.toast) store.toast.rev = store.view.rev
    }
    this.lastPut = cur
  }

  // «На все» в тосте: то же, что выбор «Все развороты» в окне — сухой прогон; уходят фото фона — вопрос в окне «Фоны»,
  // иначе сразу ставим. Один шаг отмены (ApplyBackgroundTo).
  async putAll(id: string, scope: BackdropScope, from: string, seq = store.openSeq) {
    if (this.h.busy || seq !== store.openSeq) return // другой альбом — тост не про него
    const all = this.h.allIds()
    const cur = sheetIn(store.view, from) ? from : (store.spread?.id ?? all[0])
    if (!cur) return
    const targets = bgTargets({ pick: 'all', cur, scope, sel: [], nums: [], all })
    this.h.busy = true
    try {
      const plan = (await Api.PlanBackgroundTo(targets, id)) as unknown as BgPlanInfo
      const text = bgAskText(plan, unitOf(store.view.settings))
      if (text) {
        this.h.show('bg', cur, scope)
        this.h.bgPick = 'all'
        this.h.ask = { kind: 'bgMany', id, text, targets }
      } else await this.putMany(targets, id)
    } catch (e) {
      store.notify(errText(e), 'error')
    } finally {
      this.h.busy = false
    }
  }

  // Ответ на вопрос 115: «Оставить свой» — прежний SetAlbumBg, «Как в альбоме» — SetAlbumBgToAll и тост; × — dismiss, правки нет.
  async answerAlbumOwn(all: boolean) {
    const a = this.h.ask
    if (a?.kind !== 'albumOwn') return
    this.h.close()
    if (!all) {
      await store.run(Api.SetAlbumBg(a.id, a.pages))
      return
    }
    if (await store.run(Api.SetAlbumBgToAll(a.id, a.pages))) store.notify(ownDoneText('bg', a.count, unitOf(store.view.settings)))
  }

  // «Поставить» в вопросе о фоне на набор.
  async confirmBgMany() {
    const a = this.h.ask
    if (a?.kind !== 'bgMany' || this.h.busy) return
    this.h.ask = null
    this.h.busy = true
    try {
      await this.putMany(a.targets, a.id)
    } finally {
      this.h.busy = false
    }
  }

  // Один вызов Go — один шаг отмены. Окно закрывается после успеха; ошибка оставляет его открытым.
  private async putMany(targets: GridTarget[], id: string) {
    try {
      const r = (await Api.ApplyBackgroundTo(targets, id)) as unknown as BgResult
      if (!(await store.take(r.view))) return // сбой GetView: красный тост цепочки уже показан (087)
      this.h.close()
      store.notify(bgDoneText(r.spreads, unitOf(store.view.settings)))
    } catch (e) {
      store.notify(errText(e), 'error')
    }
  }

  // Развороты (067): сначала вопрос Go (сухой прогон), без чисел для вопроса — ставим сразу. Окно закрывается,
  // когда решено ставить; ошибка прогона оставляет его открытым. Второй щелчок, пока идёт прогон, не нужен.
  private async pickSpread(item: DecorItem) {
    const c = this.h.cur
    if (!c || this.h.busy) return
    this.h.busy = true
    try {
      const plan = (await Api.PlanSpreadTpl(c.spread, c.scope, item.id)) as unknown as GridPlan
      if (this.h.cur !== c) return // окно закрыли, пока считался вопрос
      const sp = store.view.spreads.find((s) => s.id === c.spread)
      const text = putQuestion(plan, sp, item.places ?? 0)
      if (text) {
        this.h.ask = { kind: 'put', id: item.id, text }
        return
      }
      this.h.close()
      await this.putSpread(c.spread, c.scope, item.id)
    } catch (e) {
      store.notify(errText(e), 'error')
    } finally {
      this.h.busy = false
    }
  }

  // «Поставить» в вопросе: окно разворотов закрывается вместе с ним.
  async confirmPut() {
    const a = this.h.ask
    const c = this.h.cur
    if (a?.kind !== 'put' || !c) return
    this.h.close()
    await this.putSpread(c.spread, c.scope, a.id)
  }

  // Постановка: один вызов Go — места и оформление одним шагом отмены. Версия не выросла — повтор, шага нет.
  private async putSpread(spread: string, scope: BackdropScope, id: string) {
    const before = store.view.rev
    try {
      const r = (await Api.ApplySpreadTpl(spread, scope, id)) as unknown as TplResult
      if (!(await store.take(r.view))) return
      const t = putToast(store.view.rev !== before, r.left, r.missing)
      if (t.undo) store.undoToast(t.text)
      else store.notify(t.text)
    } catch (e) {
      store.notify(errText(e), 'error')
    }
  }
}
