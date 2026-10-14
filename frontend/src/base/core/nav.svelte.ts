import { noHooks, type StoreHooks } from './hooks'
import { COVER_ID, sheetIn } from '../geom/sheet'
import type { CoverView, SpreadView, View } from './types'

export const emptyView: View = {
  spreads: [],
  settings: { pageW: 300, pageH: 300, margin: 12, gap: 3, layflat: true, dpi: 300, bg: '', safe: 8 },
  folders: [],
  path: '',
  dirty: false,
  canUndo: false,
  canRedo: false,
  rev: 0,
  maxSpreadPhotos: 64,
  safe: [],
  minDPI: 200,
  lowDPI: 0,
  emptyPlaces: 0,
  textMaxW: 284,
  backdropUsed: [],
  marksRev: 0,
}

// Основа Store: альбом (view), текущий лист и переходы. Вынесено, чтобы store.svelte.ts не рос за 600 строк;
// наружу всё доступно через store.* как раньше.
export class Nav {
  view = $state<View>(emptyView)
  // Подключённый верх (hooks.ts): ставит shell/boot/wire.ts. Реактивно — ленты пересчитаются после подключения.
  hooks = $state.raw<StoreHooks>(noHooks)
  plug(h: StoreHooks) {
    this.hooks = h
  }
  current = $state(0)
  // id текущего разворота: индекс «следует» за ним при перестановках/удалениях.
  currentId = $state('') // $state: эффект ленты (Timeline) сворачивает «+N» при смене разворота

  // Когда был переход: ряд разворотов (spread/spreadrow.svelte.ts) едет только сразу после go(),
  // а не при загрузке или удалении. navSeq растёт на каждый go() — ряд видит и переход на тот же разворот
  // (со слота «новый» назад).
  navAt = 0
  navSeq = $state(0)

  // Над группой какой страницы курсор в полосе над разворотом.
  hoverPage = $state<'left' | 'right' | null>(null)

  // Обложка (032) стоит вне view.spreads, но первой в ряду холста и ленте. onCover — текущий лист она;
  // current/currentId при этом остаются у разворота, к которому вернёмся.
  onCover = $state(false)
  // В проекте-листе (031) обложки нет, даже если Go прислал её поле.
  cover = $derived<CoverView | null>(this.view.settings?.sheet ? null : (this.view.cover ?? null))
  // Листы ряда холста: [обложка?] + развороты. Индекс в ряду — sheetIndex; индекс в view.spreads — current.
  sheets = $derived<SpreadView[]>(this.cover ? [this.cover.sheet, ...this.view.spreads] : this.view.spreads)
  lead = $derived(this.cover ? 1 : 0)
  sheetIndex = $derived(this.onCover ? 0 : this.current + this.lead)
  // Неизменяемый набор, пересоздаётся целиком: SvelteSet завёл бы сигнал на каждый id.
  // eslint-disable-next-line svelte/prefer-svelte-reactivity
  used = $derived(new Set(this.sheets.flatMap((s) => s.photos)))
  spread = $derived<SpreadView | undefined>(this.onCover && this.cover ? this.cover.sheet : this.view.spreads[this.current])

  // i — индекс в view.spreads (обложку открывает goCover). Без разворотов, но с обложкой — она и открывается.
  go(i: number) {
    const next = Math.max(0, Math.min(i, this.view.spreads.length - 1))
    this.navAt = performance.now()
    this.navSeq++
    this.current = next
    this.currentId = this.view.spreads[this.current]?.id ?? ''
    this.onCover = this.view.spreads.length === 0 && !!this.cover
    this.hoverPage = null
    this.hooks.went()
  }

  // Обложка — текущий лист; разворот, на котором стояли, помнится.
  goCover() {
    if (!this.cover) return
    this.navAt = performance.now()
    this.navSeq++
    this.onCover = true
    this.hoverPage = null
    this.hooks.went()
  }

  // i — индекс в ряду холста (sheets): 0 при обложке — она.
  goSheet(i: number) {
    if (this.lead && i <= 0) this.goCover()
    else this.go(i - this.lead)
  }

  goId(id: string) {
    if (id === COVER_ID) return this.goCover()
    const i = this.view.spreads.findIndex((s) => s.id === id)
    if (i >= 0) this.go(i)
  }

  // Лист по id: развороты и обложка (для жестов и правок, которым достаточно id листа).
  sheet(id: string): SpreadView | undefined {
    return sheetIn(this.view, id)
  }
}
