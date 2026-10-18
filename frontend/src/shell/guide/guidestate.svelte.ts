// Справочник функций (спека 033): состояние окна и исполнение плана показа. Решения — в show.ts и
// spot.ts (под тестами); здесь только вызовы store/prefs/modes. Проект не меняется ни одним шагом:
// правятся уровень, лист, выделение и панели.
import { tick } from 'svelte'
import { store } from '../../base/core/store.svelte'
import { prefs } from '../../base/prefs/prefs.svelte'
import { keymap } from '../../base/keys/keymap.svelte'
import { modes } from '../../base/prefs/view.svelte'
import type { RowId } from '../../base/prefs/levels'
import { isSheet } from '../../base/geom/sheet'
import type { SpreadView } from '../../base/core/types'
import { bgmode } from '../../editor/backdrop/bgmode.svelte'
import { canvas } from '../../editor/canvas/canvas.svelte'
import { photoui } from '../../editor/photo/photoui.svelte'
import { placesel } from '../../editor/places/placesel.svelte'
import { edit } from '../../editor/text/edit.svelte'
import { gallery } from '../../panels/gallery/gallery.svelte'
import { isVignette } from '../../panels/vignette/vignette'
import { menuUi } from '../menu/menuui.svelte'
import { articleById, placesOf, type Article } from './articles'
import { planShow, spotId, type Plan, type SheetInfo, type ShowCtx, type Step } from './show'
import { choose, locate, spot } from './spotstate.svelte'
import { joinToast, missToast } from './spot'

const info = (s: SpreadView): SheetInfo => ({
  kind: isVignette(s) ? 'vignette' : '',
  grid: s.grid,
  photos: s.photos.length,
  texts: s.texts.length,
})

// Шаги плана → действия над видом. Проект не меняется ни одним: правятся уровень, лист, выделение и панели.
type Runner = { [K in Step['do']]: (s: Extract<Step, { do: K }>) => void }
const RUN: Runner = {
  level: (s) => void prefs.set(s.to),
  custom: (s) => void prefs.toggle(s.row, true),
  unbig: () => {
    if (modes.big) modes.toggleBig()
  },
  bgExit: () => bgmode.exit(),
  closeGallery: () => gallery.close(),
  closeMenus: () => {
    photoui.menu = null
    photoui.preview = null
  },
  cover: () => store.goCover(),
  go: (s) => store.go(s.index),
  selectPhoto: () => {
    const p = store.spread?.photos[0]
    if (p) store.selectedCell = p
  },
  selectLibrary: () => {
    const p = store.visible[0]
    if (p) store.click(p.id, new MouseEvent('click')) // как щелчок по первой миниатюре
  },
  selectText: () => {
    const sp = store.spread
    if (sp?.texts[0]) edit.select({ kind: 'text', spread: sp.id, id: sp.texts[0].id })
  },
  clearSel: () => {
    edit.clear()
    placesel.select([])
  },
  details: (s) => modes.openSection(s.section),
  gallery: () => gallery.open('spread'),
  fit: () => canvas.fit(),
  menu: (s) => menuUi.ask(s.menu),
}

class Guide {
  open = $state(false)
  // Запрос и выбор сохраняются между открытиями.
  query = $state('')
  sel = $state('')
  // Список ограничен функциями одной строки таблицы уровней (из окна «Уровни и функции»).
  rowFilter = $state<RowId | null>(null)
  private seq = 0

  // F1, меню, ссылка в стартовом окне: прежние запрос и выбор.
  openFresh() {
    this.rowFilter = null // фильтр строки — только от «?» матрицы, к F1 он не липнет
    this.open = true
  }

  // «?» у строки матрицы уровней: поиск очищен, список — функции этой строки.
  openRow(row: RowId) {
    this.query = ''
    this.rowFilter = row
    this.open = true
  }

  // Состояние окна для плана: читается в момент вызова (карточка пересчитывает его реактивно).
  ctx(): ShowCtx {
    const sp = store.spread
    const sel = store.selectedCell
    return {
      level: prefs.level,
      custom: prefs.custom,
      can: prefs.can,
      sheet: isSheet(store.view.settings),
      spreads: store.view.spreads.map(info),
      index: store.view.spreads.length > 0 ? store.current : -1,
      cover: store.cover ? info(store.cover.sheet) : null,
      onCover: store.onCover,
      selPhoto: !!sel && !!sp?.photos.includes(sel),
      selText: edit.sel?.kind === 'text' && edit.sel.spread === sp?.id,
      selAny: placesel.refs.length > 0 || placesel.texts.length > 0 || placesel.stickers.length > 0 || !!edit.sel,
      libPhotos: store.visible.length,
      libSel: store.selectedIds.length > 0,
      cardBody: !!sp,
      big: modes.big,
      bgMode: !!bgmode.cur,
      galleryOpen: !!gallery.cur,
      menusOpen: !!photoui.menu || !!photoui.preview,
      zoomed: canvas.zoomed,
    }
  }

  private exec(s: Step) {
    ;(RUN[s.do] as (s: Step) => void)(s)
  }

  // «Показать»: окна закрываются, интерфейс открывает нужное, место мигает. spotKey — место из кнопки карточки.
  async show(id: string, spotKey?: string) {
    const a = articleById(id)
    if (!a) return
    const my = ++this.seq
    spot.cancel() // прежняя подсветка (и её закрытие меню) — до новых шагов, иначе закроет только что открытое меню
    this.open = false
    if (prefs.startOpen) prefs.closeStart()
    prefs.compare = false
    const ctx = this.ctx()
    const plan = planShow(a, ctx, spotKey ? placesOf(a).find((p) => p.spot === spotKey) : undefined)
    const prev = prefs.snapshot()
    for (const s of plan.steps) this.exec(s)
    this.announce(plan, prev)
    await tick()
    if (my !== this.seq) return
    await this.flash(a, plan, prev)
  }

  // Тост об изменении уровня с «Вернуть»; без него — тост плана (мигать нечем).
  private announce(plan: Plan, prev: ReturnType<typeof prefs.snapshot>) {
    if (!plan.toast) return
    const changed = plan.steps.some((s) => s.do === 'level' || s.do === 'custom')
    if (changed) store.notify(plan.toast, 'info', { label: 'Вернуть', run: () => void prefs.apply(prev) })
    else if (!plan.spot) store.notify(plan.toast)
  }

  // Ищет место: основное, затем «⋯» полосы фото, затем запасное статьи (miss.spot).
  private async find(a: Article, spotKey: string, planHint: string): Promise<{ el: HTMLElement; hint: string } | null> {
    const id = spotId(spotKey)
    const isMenu = spotKey.startsWith('menu:')
    const sel = isMenu ? `[role="menu"] [data-item="${id}"]` : `[data-guide~="${id}"]`
    if (!isMenu) spot.reveal(sel) // кнопки, видимые только при наведении, раскрываются до поиска
    let els = await locate(sel)
    let hint = planHint
    if (els.length === 0 && !isMenu) {
      // Кнопка ушла в «⋯» полосы фото: мигает «⋯».
      els = await locate(`[data-guide-hold~="${id}"]`, 100)
      hint = els.length > 0 ? 'В меню «⋯»' : ''
    }
    const el = choose(els)
    if (el) return { el, hint }
    const alt = a.miss?.spot
    if (alt && !isMenu) {
      const s2 = `[data-guide~="${spotId(alt)}"]`
      spot.reveal(s2)
      const e2 = choose(await locate(s2, 300))
      if (e2) return { el: e2, hint: keymap.text(a.miss?.hint ?? '') }
    }
    return null
  }

  private async flash(a: Article, plan: Plan, prev: ReturnType<typeof prefs.snapshot>) {
    if (!plan.spot) {
      if (!plan.toast) store.notify(missToast(a)) // мигать нечем и тоста нет: всё равно говорим, где это
      return
    }
    const found = await this.find(a, plan.spot, plan.hint ?? '')
    if (found) {
      spot.flash(found.el, a.title, found.hint, plan.spot.startsWith('menu:') ? () => menuUi.close() : undefined)
      return
    }
    spot.cancel()
    // Не нашли: тост «Название: где» — всегда. Тост о смене уровня не теряем: он идёт в том же сообщении.
    const miss = missToast(a, plan.hint ?? plan.miss ?? a.miss?.hint)
    const changed = plan.steps.some((s) => s.do === 'level' || s.do === 'custom')
    if (changed && plan.toast) store.notify(joinToast(plan.toast, miss), 'info', { label: 'Вернуть', run: () => void prefs.apply(prev) })
    else store.notify(miss)
  }
}

export const guide = new Guide()
