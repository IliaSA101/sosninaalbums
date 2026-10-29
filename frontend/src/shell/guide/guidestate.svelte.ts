// Справочник функций (спека 033): состояние окна и исполнение плана показа. Решения — в show.ts и
// spot.ts (под тестами); здесь только вызовы store/prefs/modes. Проект не меняется ни одним шагом:
// правятся уровень, лист, выделение и панели.
import { tick } from 'svelte'
import { SvelteSet } from 'svelte/reactivity'
import { store } from '../../base/core/store.svelte'
import { prefs } from '../../base/prefs/prefs.svelte'
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
import { articleById, placesOf, type Article, type GroupId, type SpotKey } from '../guidetext/articles'
import { planShow, type Plan, type SheetInfo, type ShowCtx, type Step } from './show'
import { seek, spot } from './spotstate.svelte'
import { articleText } from '../guidetext/articletext'
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
    photoui.closePreview()
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

// Один шаг плана: тур первых шагов (113) двигает вид теми же шагами, что и «Показать».
export function runStep(s: Step): void {
  ;(RUN[s.do] as (s: Step) => void)(s)
}

class Guide {
  open = $state(false)
  // «Показать» ещё ведёт план (шаги вида, поиск места): тур в это время кольцо не берёт.
  busy = $state(false)
  // Запрос и выбор сохраняются между открытиями.
  query = $state('')
  sel = $state('')
  // Список ограничен функциями одной строки таблицы уровней (из окна «Уровни и функции»).
  rowFilter = $state<RowId | null>(null)
  // Разделы списка, открытые руками (136): живут до закрытия программы, в config.json не пишутся.
  // Раздел выбранной статьи открыт всегда, независимо от этого набора.
  readonly opened = new SvelteSet<GroupId>()
  private seq = 0

  // F1, меню, ссылка в стартовом окне: прежние запрос и выбор.
  openFresh() {
    this.rowFilter = null // фильтр строки — только от «?» матрицы, к F1 он не липнет
    this.open = true
  }

  // Тур и ссылки «Подробнее в справочнике»: окно открывается на статье, запрос пуст.
  openArticle(id: string) {
    if (!articleById(id)) return
    this.rowFilter = null
    this.query = ''
    this.sel = id
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
      libAll: store.photos.length,
      libSel: store.selectedIds.length > 0,
      cardBody: !!sp,
      big: modes.big,
      bgMode: !!bgmode.cur,
      galleryOpen: !!gallery.cur,
      menusOpen: !!photoui.menu || !!photoui.preview,
      zoomed: canvas.zoomed,
    }
  }

  // «Показать»: окна закрываются, интерфейс открывает нужное, место мигает. spotKey — место из кнопки карточки.
  async show(id: string, spotKey?: string) {
    const a = articleById(id)
    if (!a) return
    const my = ++this.seq
    this.busy = true
    try {
      await this.run(a, spotKey, my)
    } finally {
      if (my === this.seq) this.busy = false
    }
  }

  private async run(a: Article, spotKey: string | undefined, my: number) {
    spot.cancel() // прежняя подсветка (и её закрытие меню) — до новых шагов, иначе закроет только что открытое меню
    this.open = false
    if (prefs.startOpen) prefs.closeStart()
    prefs.compare = false
    const ctx = this.ctx()
    const plan = planShow(a, ctx, spotKey ? placesOf(a).find((p) => p.spot === spotKey) : undefined)
    const prev = prefs.snapshot()
    for (const s of plan.steps) runStep(s)
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

  // Ищет место (seek), затем запасное статьи (miss.spot).
  private async find(a: Article, spotKey: string, planHint: string): Promise<{ el: HTMLElement; hint: string } | null> {
    const found = await seek(spotKey as SpotKey, planHint)
    if (found) return found
    const alt = a.miss?.spot
    if (alt && !spotKey.startsWith('menu:')) {
      const f2 = await seek(alt, articleText(a.miss?.hint ?? ''))
      if (f2) return f2
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
