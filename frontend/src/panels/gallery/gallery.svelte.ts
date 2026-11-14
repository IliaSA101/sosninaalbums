// Галерея раскладок (спека 003/007): состояние, предпросмотр по наведению и запрос вариантов у Go.
// Вынесено из store.svelte.ts; store зовёт sync() из apply и close() из go().
import * as Api from '../../../wailsjs/go/app/App'
import { errText } from '../../base/words/errtext'
import { store } from '../../base/core/store.svelte'
import { gridAct } from '../layouts/gridact.svelte'
import { coverAct } from '../layouts/coverbar'
import { COVER_ID, hasSeam, isCover } from '../../base/geom/sheet'
import { startOrder, type FamilyFilter, type RowOrder } from './variants'
import type { TplSource } from '../../base/tpl/tplsource'
import type { VigFamilyFilter } from '../../base/tpl/tplfilter'
import type { FormFilter } from '../vignette/vigform'
import { stepVignette } from '../vignette/vigact'
import { isVignette } from '../vignette/vignette'
import type { Choices, CoverSide, GalleryTab, LayoutScope, PageView, RotRect, SpreadView } from '../../base/core/types'

// Сторонка обложки (127) с этим scope из вида Go; обложки или сторонки нет — undefined.
function coverSide(scope: LayoutScope): CoverSide | undefined {
  return store.cover?.sides?.find((x) => x.scope === scope)
}

// У страницы есть что выбирать: ⊞, листание и галерея страницы. Раскладка вне рядов
// (variant −1) при ≥ 2 фото тоже считается — листание вернёт её в ряд.
export function pageSwitchable(p: PageView | undefined): boolean {
  return !!p && (p.variants >= 2 || (p.variant < 0 && p.photos >= 2))
}

// Страница, на которую открыта галерея, пропала: у листа нет шва (одна страница) или выбирать на ней нечего.
function pageGone(s: SpreadView, scope: LayoutScope): boolean {
  return scope !== 'spread' && !(hasSeam(s) && pageSwitchable(s.pages?.[scope === 'left' ? 0 : 1]))
}

class Gallery {
  // Открытая галерея и предпросмотр по наведению на карточку (Go не вызывается).
  // tab: 'layouts' — раскладки «Авто» (LayoutGallery); 'grids' — сетки (спека 007, GridGallery, U9):
  // открывается и у «Авто», и у сетки, варианты запрашивает сама вкладка (GridChoices), а не sync.
  cur = $state<{ spread: string; scope: LayoutScope; tab: GalleryTab } | null>(null)
  choices = $state<Choices | null>(null)
  preview = $state<{ spread: string; rects: RotRect[] } | null>(null) // rot — у шаблонов с углом

  moreBusy = $state(false) // плитка «Ещё» ждёт ответа Go

  // 041: фильтр семейств и ряды, где похожие раскрыты (по подписи ряда). Живут, пока галерея открыта.
  family = $state<FamilyFilter>('all')
  openTwins = $state<readonly string[]>([])
  // 050: ряд «Вариантов» и источник «Шаблонов» и виньетки. Живут, пока галерея открыта.
  order = $state<RowOrder>('inOrder')
  source = $state<TplSource>('all')
  // 049: фильтр видов шаблонов виньетки (▦); здесь, а не в VigGallery, чтобы Пробел листал видимые.
  vigFamily = $state<VigFamilyFilter>('all')
  // 056: фильтр формы мест рядом с ним; только отбирает плитки, сбрасывается вместе с ним
  vigForm = $state<FormFilter>('all')

  private choicesReq = 0
  private choicesWant = ''

  // Листание по кругу: всего разворота или одной страницы (вторая не двигается).
  async step(scope: LayoutScope, delta: number) {
    const s = store.spread
    // Виньетка: листаются шаблоны виньетки, а не раскладки «Авто» (решение 80).
    if (isVignette(s)) {
      await stepVignette(s, delta, this.vigFamily, this.vigForm)
      return
    }
    if (!s || !s.layout || isCover(s)) return
    await store.run(Api.StepLayout(s.id, scope, delta))
  }

  // Листание раскладок сторонки обложки (127: ‹ ›, Пробел). Решает coverAct по виду Go (0.26-127-15):
  // места не узнаны или их нет — открыть ▦ (повторно её не закрываем); в ряду одна раскладка — тост;
  // иначе Go листает «По фото» по кругу, серия — один шаг отмены.
  async coverStep(scope: LayoutScope, delta: number) {
    const side = coverSide(scope)
    if (!side) return
    const act = coverAct(side)
    if (act.kind === 'toast') store.notify(act.text)
    else if (act.kind === 'open') {
      const g = this.cur
      if (!(g?.spread === COVER_ID && g.scope === scope && g.tab === 'grids')) this.open(scope, 'grids')
    } else await store.run(Api.StepCoverGrid(scope, delta))
  }

  // Повторный вызов той же области закрывает галерею, другой — переключает на неё.
  // Вкладка 'grids' открывается у любого разворота (область — 'spread', её меняет сама вкладка),
  // 'layouts' — только у «Авто». У обложки (127) — только 'grids' и только на left/right: галерея сторонки.
  open(scope: LayoutScope, tab: GalleryTab = 'layouts') {
    const s = store.spread
    // На сетке раскладки открываются тоже: выбор спросит «Вернуть авто?» (backToAuto).
    // У обложки (032) раскладок «Авто» и копирования сетки нет; есть галерея сторонки (127).
    if (!s) return
    if (isCover(s)) {
      if (tab !== 'grids' || !coverSide(scope)) return
    } else if (tab === 'layouts' && !s.layout && !s.grid) return
    // У виньетки одна галерея — шаблоны виньетки (LayoutGallery подменяет содержимое); шаблонов сетки нет.
    if (isVignette(s)) {
      if (tab !== 'layouts') return
      scope = 'spread'
    }
    const g = this.cur
    if (g?.spread === s.id && g.scope === scope && g.tab === tab) return this.close()
    this.cur = { spread: s.id, scope, tab }
    this.preview = null
    this.resetView(startOrder(s, scope))
    this.sync()
  }

  close() {
    this.cur = null
    this.preview = null
    this.resetView('inOrder')
  }

  private resetView(order: RowOrder) {
    this.family = 'all'
    this.openTwins = []
    this.source = 'all'
    this.vigFamily = 'all'
    this.vigForm = 'all'
    this.order = order
  }

  setOrder(o: RowOrder) {
    this.order = o
  }

  setSource(s: TplSource) {
    this.source = s
  }

  // Смена области на вкладке «Шаблоны»: галерея остаётся открытой, источник и прочее состояние вида не сбрасываются.
  rescope(scope: LayoutScope) {
    const g = this.cur
    if (!g || g.scope === scope) return
    this.cur = { ...g, scope }
    this.preview = null
  }

  setVigFamily(f: VigFamilyFilter) {
    this.vigFamily = f
  }

  setVigForm(f: FormFilter) {
    this.vigForm = f
  }

  setFamily(f: FamilyFilter) {
    this.family = f
  }

  // Плитка «Похожие: N»: раскрыть ряд на месте и обратно.
  toggleTwins(row: string) {
    this.openTwins = this.openTwins.includes(row) ? this.openTwins.filter((r) => r !== row) : [...this.openTwins, row]
  }

  // Нужны только rects: ими же показывают и шаблоны сетки (с углом), не целый Choice.
  previewChoice(c: { key: string; rects: RotRect[] } | null) {
    const g = this.cur
    this.preview = c && g ? { spread: g.spread, rects: c.rects } : null
  }

  async setLayout(key: string) {
    const g = this.cur
    if (!g) return
    this.close()
    // Виньетка не возвращается в «Авто» обычной раскладкой — выход только «Обычный разворот».
    if (isVignette(store.view.spreads.find((x) => x.id === g.spread))) return
    // На сетке выбор раскладки сбрасывает места — сначала вопрос.
    await gridAct.backToAuto(g.spread, key)
  }

  // Плитка «Ещё» (027): Go досчитывает ряд «Авто» на порцию. Ответ годится, если проект не менялся
  // и галерея всё ещё на этом развороте; иначе его выбрасываем (sync уже запросил свежие варианты).
  async loadMore() {
    const g = this.cur
    const c = this.choices
    if (!g || !c || c.spread !== g.spread || this.moreBusy) return
    const have = c.whole.inOrder.length
    this.moreBusy = true
    try {
      const res = (await Api.MoreLayouts(g.spread, have)) as unknown as Choices
      const cur = this.cur
      if (cur?.spread === g.spread && res.spread === g.spread && res.rev === store.view.rev) this.choices = res
    } catch (e) {
      store.notify(errText(e), 'error')
    } finally {
      this.moreBusy = false
    }
  }

  // Галерея следует за проектом: разворот исчез или страница больше не своя — закрыть;
  // проект изменился (другой rev) — перезапросить варианты.
  sync() {
    const g = this.cur
    if (!g) return
    // Обложки нет в View.spreads: галерея сторонки живёт, пока в виде есть эта сторонка.
    if (g.spread === COVER_ID) return coverSide(g.scope) ? undefined : this.close()
    const s = store.view.spreads.find((x) => x.id === g.spread)
    // Шаблоны виньетки галерея запрашивает сама (VigGallery): тут только следим, чтобы лист был на месте.
    if (isVignette(s)) return
    // Вкладке сеток варианты не нужны (GridChoices запрашивает она сама): только следим, чтобы разворот был на месте.
    if (g.tab === 'grids') return s ? undefined : this.close()
    if (!s || (!s.layout && !s.grid) || pageGone(s, g.scope)) return this.close()
    const c = this.choices
    if (c?.spread === g.spread && c.rev === store.view.rev) return
    // Старые прямоугольники к новому составу не подходят.
    this.preview = null
    const want = `${g.spread}:${store.view.rev}`
    if (this.choicesWant === want) return
    this.choicesWant = want
    const req = ++this.choicesReq
    Api.LayoutChoices(g.spread)
      .then((res) => {
        if (req !== this.choicesReq) return
        this.choices = res as unknown as Choices
        const cur = this.cur
        if (cur && cur.scope !== 'spread' && !this.choices[cur.scope]) this.close()
      })
      .catch((e) => {
        if (req === this.choicesReq) store.notify(errText(e), 'error')
      })
      .finally(() => {
        if (req === this.choicesReq) this.choicesWant = ''
      })
  }
}

export const gallery = new Gallery()
