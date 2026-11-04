<script lang="ts">
  import { store, Api } from '../../base/core/store.svelte'
  import { gallery } from './gallery.svelte'
  import { errText } from '../../base/words/errtext'
  import { tplio } from '../../shell/menu/tplio.svelte'
  import { applyChoice } from '../layouts/gridactions'
  import { builtinOf, cardBoxes, defaultN, dropTitle, gridSourceTexts, mineOf, N_ITEMS, previewRects } from './gridgallery'
  import type { GridChoice, GridChoices, LayoutScope } from '../../base/core/types'
  import { hasBorders, isSheet, newSheetSize, sheetIn, sheetSize, unitOf } from '../../base/geom/sheet'
  import GridTplModals from './GridTplModals.svelte'
  import TplRow from '../../base/tpl/TplRow.svelte'
  import TplItem from '../../base/tpl/TplItem.svelte'
  import TplSep from '../../base/tpl/TplSep.svelte'
  import SideList from '../../base/tpl/SideList.svelte'
  import Seg from '../../base/tpl/Seg.svelte'
  import { tplSources } from '../../base/tpl/tplsource'
  import GalleryHead, { galleryTabs } from './GalleryHead.svelte'
  import { liveRadius, shapeCss } from '../../base/geom/shape'

  // Вкладка «Шаблоны» галереи (спека 007): встроенные сетки на n фото и «Мои шаблоны»; шапка и переключатель ряда — 050.
  // Доли boxes от Go — проценты карточки; интерфейс ничего не считает, кроме перевода в мм для предпросмотра.
  const H = 80 // высота карточки
  // g приходит от LayoutGallery: пока галерея уезжает, gallery.cur уже null.
  let { g }: { g: { spread: string; scope: LayoutScope } } = $props()
  const s = $derived(store.view.settings)
  const unit = $derived(unitOf(s))
  const sp = $derived(store.view.spreads.find((x) => x.id === g.spread))
  const scope = $derived(g.scope)
  // Размер листа и шов — из Go (sheet.ts); нет листа — по настройкам альбома.
  const sheet = $derived(sp ? sheetSize(sp) : newSheetSize(s))
  const cardW = $derived((H * (scope === 'spread' ? sheet.w : sheet.mid)) / sheet.h)

  // Фильтр: пока не выбран руками — по умолчанию (число фото или мест в области).
  let nPick = $state<number | null>(null)
  const nWant = $derived(nPick ?? (sp ? defaultN(sp, sheet.mid, scope) : 1))
  $effect(() => {
    void scope // другая область — снова фильтр по умолчанию
    nPick = null
  })

  // Ответ Go; ключ — для какого разворота, фильтра и области он посчитан.
  let c = $state<{ key: string; scope: LayoutScope; res: GridChoices } | null>(null)
  let loading = $state(false)
  let again = $state(0) // перезапрос после правки шаблонов
  let req = 0
  const key = $derived(`${g.spread}|${nWant}|${scope}`)
  $effect(() => {
    const k = key
    const sc = scope
    void [again, store.view.rev, tplio.seq] // tplio.seq — шаблоны загрузили из файла (086)
    const my = ++req
    loading = true
    Api.GridChoices(g.spread, nWant, scope)
      .then((res) => {
        if (my === req) c = { key: k, scope: sc, res: res as unknown as GridChoices }
      })
      .catch((e) => {
        if (my === req) store.notify(errText(e), 'error')
      })
      .finally(() => {
        if (my === req) loading = false
      })
  })
  const res = $derived(c && c.key === key ? c.res : null)
  const mine = $derived(mineOf(res, scope))
  // Какой ряд показать — «Все» (встроенные, за чертой свои) / «Встроенные» / «Мои»; живёт в gallery.source.
  const texts = $derived(gridSourceTexts(scope, nWant, unit.one))
  const view = $derived(tplSources(builtinOf(res, scope), mine, gallery.source, texts))
  // Пока ответ на новый фильтр не пришёл, переключатель считает по прежнему ответу: иначе пункты на миг серые с ложной причиной
  // Первая загрузка (ответа ещё не было): пункты без чисел и не серые.
  const sideItems = $derived(
    res
      ? view.items
      : c
        ? tplSources(builtinOf(c.res, c.scope), mineOf(c.res, c.scope), gallery.source, texts).items
        : view.items.map(({ id, label }) => ({ id, label })),
  )
  const tiles = $derived(view.parts.reduce((n, p) => n + p.list.length, 0))

  // У листа виньетки страниц нет (Go отказывает шаблонам по страницам): область одна, выбор прячется.
  const pages = $derived<{ id: LayoutScope; label: string }[]>(
    isSheet(s)
      ? [{ id: 'spread', label: 'Лист' }]
      : [
          { id: 'spread', label: 'Разворот' },
          { id: 'left', label: 'Левая' },
          { id: 'right', label: 'Правая' },
        ],
  )
  // Галерея остаётся открытой, источник не сбрасывается (rescope, а не open)
  const setScope = (next: LayoutScope) => gallery.rescope(next)

  // Пока галерея уезжает (slide), компонент уже inert: чтение его $derived (scope, sheet) в обработчике даёт
  // derived_inert, а повторный щелчок применил бы шаблон дважды. Закрыта — обработчики ничего не читают.
  function hover(ch: GridChoice | null) {
    if (!gallery.cur) return
    gallery.previewChoice(ch ? { key: ch.id, rects: previewRects(ch, scope, sheet) } : null)
  }
  function pick(ch: GridChoice) {
    if (!gallery.cur) return
    const { spread } = g
    const sc = scope
    gallery.close()
    void applyChoice(spread, sc, ch)
  }

  // ---------- шаблоны: переименовать, удалить, сохранить — окна общие с обложкой (GridTplModals) ----------
  let modals: ReturnType<typeof GridTplModals>
  const saveHint = $derived(`Запоминаются только места, без фото.${pages.length > 1 ? ' «Левая» и «Правая» дают шаблон страницы.' : ''}`)
</script>

<GalleryHead
  tabs={galleryTabs()}
  title={loading ? 'Считаю…' : ''}
  save={{
    title: isSheet(s) ? 'Запомнить места этого листа как шаблон' : 'Запомнить места этого разворота или страницы как шаблон',
    onclick: () => modals.openSave(scope),
  }}
>
  {#snippet filters()}
    {#if pages.length > 1}<Seg items={pages} value={scope} onpick={setScope} label="Область" />{/if}
    <Seg items={N_ITEMS} value={nWant} onpick={(n) => (nPick = n)} label="Число мест" />
  {/snippet}
</GalleryHead>

<TplRow count={tiles} empty={loading && !res ? '' : view.empty} inert={loading} watch={[scope, nWant, gallery.source]}>
  {#snippet side()}
    <SideList items={sideItems} value={gallery.source} onpick={(x) => gallery.setSource(x)} label="Какие шаблоны показать" />
  {/snippet}
  {#each view.parts as p, pi (pi)}
    {#if p.sep}<TplSep label={p.sep} />{/if}
    {#each p.list as ch (ch.id)}
      {@const own = p.list === mine}
      <TplItem
        name={own ? ch.name : undefined}
        onrename={own ? () => (modals.openRename(ch), hover(null)) : undefined}
        ondelete={own ? () => (modals.openDelete(ch), hover(null)) : undefined}
      >
        <button
          class="card"
          class:dim={loading}
          style:width="{cardW}px"
          style:height="{H}px"
          title={ch.name || undefined}
          onmouseenter={() => hover(ch)}
          onmouseleave={() => hover(null)}
          onclick={() => pick(ch)}
        >
          {#each cardBoxes(ch, scope) as r, i (i)}
            <span
              class="cell"
              style:left="{r.x * 100}%"
              style:top="{r.y * 100}%"
              style:width="{r.w * 100}%"
              style:height="{r.h * 100}%"
              style:transform={r.rot ? `rotate(${r.rot}deg)` : undefined}
              style:border-radius={shapeCss(r.shape, liveRadius(r.round ?? 0, r.w * cardW, r.h * H), r.w * cardW, r.h * H)}
            ></span>
          {/each}
          <span class="spine"></span>
          {#if ch.drop > 0}<span class="drop" title={dropTitle(ch.drop)}>−{ch.drop}</span>{/if}
        </button>
      </TplItem>
    {/each}
  {/each}
</TplRow>

<GridTplModals
  bind:this={modals}
  spread={g.spread}
  areas={pages}
  hint={saveHint}
  borders={(sc) => hasBorders(sheetIn(store.view, g.spread), sc)}
  applied={unit.cap === 'Лист' ? 'Листы' : 'Развороты'}
  ondone={() => again++}
/>

<style>
  .card {
    all: unset;
    display: block;
    position: relative;
    overflow: clip; /* повёрнутая коробка у края не вылезает из карточки */
    background: #fff;
    cursor: pointer;
    outline: 2px solid transparent;
    outline-offset: 2px;
    transition:
      outline-color 0.15s,
      opacity 0.15s;
  }
  .card:hover {
    outline-color: var(--accent);
  }
  .card.dim {
    opacity: 0.45;
  }
  .cell {
    position: absolute;
    background: #d8d8d8;
    box-shadow: inset 0 0 0 1px #a8a8a8;
  }
  .spine {
    position: absolute;
    left: 50%;
    top: 0;
    bottom: 0;
    width: 1px;
    background: rgba(0, 0, 0, 0.25);
  }
  /* «−N»: столько фото уйдёт в ленту при применении */
  .drop {
    position: absolute;
    right: 2px;
    bottom: 2px;
    padding: 0 4px;
    border-radius: 3px;
    background: #c0392b;
    color: #fff;
    font-size: 11px;
    font-weight: 700;
    line-height: 16px;
  }
</style>
