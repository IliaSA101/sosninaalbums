// Галерея раскладок (спека 003/007): состояние, предпросмотр по наведению и запрос вариантов у Go.
// Вынесено из store.svelte.ts; store зовёт sync() из apply и close() из go().
import * as Api from '../../../wailsjs/go/app/App'
import { errText } from '../../base/core/errtext'
import { store } from '../../base/core/store.svelte'
import { gridAct } from './gridact.svelte'
import type { Choice, Choices, GalleryTab, LayoutScope, PageView, RotRect } from '../../base/core/types'

// У страницы есть что выбирать: ⊞, листание и галерея страницы. Раскладка вне рядов
// (variant −1) при ≥ 2 фото тоже считается — листание вернёт её в ряд.
export function pageSwitchable(p: PageView | undefined): boolean {
  return !!p && (p.variants >= 2 || (p.variant < 0 && p.photos >= 2))
}

class Gallery {
  // Открытая галерея и предпросмотр по наведению на карточку (Go не вызывается).
  // tab: 'layouts' — раскладки «Авто» (LayoutGallery); 'grids' — сетки (спека 007, GridGallery, U9):
  // открывается и у «Авто», и у сетки, варианты запрашивает сама вкладка (GridChoices), а не sync.
  cur = $state<{ spread: string; scope: LayoutScope; tab: GalleryTab } | null>(null)
  choices = $state<Choices | null>(null)
  preview = $state<{ spread: string; rects: RotRect[] } | null>(null) // rot — у шаблонов с углом

  private choicesReq = 0
  private choicesWant = ''

  // Листание по кругу: всего разворота или одной страницы (вторая не двигается).
  async step(scope: LayoutScope, delta: number) {
    const s = store.spread
    if (!s || !s.layout) return
    await store.run(Api.StepLayout(s.id, scope, delta))
  }

  // Повторный вызов той же области закрывает галерею, другой — переключает на неё.
  // Вкладка 'grids' открывается у любого разворота (область — 'spread', её меняет сама вкладка),
  // 'layouts' — только у «Авто».
  open(scope: LayoutScope, tab: GalleryTab = 'layouts') {
    const s = store.spread
    // На сетке раскладки открываются тоже: выбор спросит «Вернуть авто?» (backToAuto).
    if (!s || (tab === 'layouts' && !s.layout && !s.grid)) return
    const g = this.cur
    if (g?.spread === s.id && g.scope === scope && g.tab === tab) return this.close()
    this.cur = { spread: s.id, scope, tab }
    this.preview = null
    this.sync()
  }

  close() {
    this.cur = null
    this.preview = null
  }

  previewChoice(c: Choice | null) {
    const g = this.cur
    this.preview = c && g ? { spread: g.spread, rects: c.rects } : null
  }

  async setLayout(key: string) {
    const g = this.cur
    if (!g) return
    this.close()
    // На сетке выбор раскладки сбрасывает места — сначала вопрос.
    await gridAct.backToAuto(g.spread, key)
  }

  // Галерея следует за проектом: разворот исчез или страница больше не своя — закрыть;
  // проект изменился (другой rev) — перезапросить варианты.
  sync() {
    const g = this.cur
    if (!g) return
    const s = store.view.spreads.find((x) => x.id === g.spread)
    // Вкладке сеток варианты не нужны (GridChoices запрашивает она сама): только следим, чтобы разворот был на месте.
    if (g.tab === 'grids') return s ? undefined : this.close()
    const page = g.scope === 'left' ? 0 : 1
    if (!s || (!s.layout && !s.grid) || (g.scope !== 'spread' && !pageSwitchable(s.pages?.[page]))) return this.close()
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
