<script lang="ts">
  import { tick } from 'svelte'
  import { store } from '../../base/core/store.svelte'
  import { gallery } from './gallery.svelte'
  import { plural } from '../../base/core/plural'
  import { newSheetSize, sheetSize } from '../../base/geom/sheet'
  import Seg from '../../base/tpl/Seg.svelte'
  import SideList from '../../base/tpl/SideList.svelte'
  import TplRow from '../../base/tpl/TplRow.svelte'
  import TplSep from '../../base/tpl/TplSep.svelte'
  import GalleryHead, { galleryName, galleryTabs } from './GalleryHead.svelte'
  import type { Choice, ChoiceRows, LayoutScope } from '../../base/core/types'
  import {
    activeFamily,
    familiesIn,
    galleryCount,
    galleryLines,
    galleryOrders,
    galleryRow,
    ROW,
    twinsLabel,
    type FamilyFilter,
  } from './variants'

  // Вкладка «Варианты» (050): шапка в одну строку и один ряд плиток; какой ряд — решает переключатель слева.
  // g приходит от LayoutGallery: пока галерея уезжает, gallery.cur уже null.
  let { g }: { g: { spread: string; scope: LayoutScope } } = $props()

  const H = 80 // высота плитки
  const sp = $derived(store.view.spreads.find((x) => x.id === g.spread))
  // Плитка — пропорции листа (Go: sv.w × sv.h); нет листа — по настройкам.
  const sheet = $derived(sp ? sheetSize(sp) : newSheetSize(store.view.settings))
  const cardW = $derived((H * sheet.w) / sheet.h)
  // Варианты показываем, только если они посчитаны для этого разворота.
  const c = $derived(gallery.choices?.spread === g.spread ? gallery.choices : null)
  const rows = $derived<ChoiceRows | null>(c ? (g.scope === 'spread' ? c.whole : c[g.scope]) : null)
  const inOrder = $derived(rows?.inOrder ?? [])
  const rearranged = $derived(rows?.rearranged ?? [])
  // 041: «Другие варианты» — только у разворота целиком (у страницы Go их не считает).
  const extra = $derived(g.scope === 'spread' ? (rows?.extra ?? []) : [])
  // У разворота — столько же, сколько листает счётчик «Раскладка N / M» (без «С перестановкой»).
  const k = $derived(galleryCount(g.scope === 'spread', { inOrder, rearranged, extra }))
  const page = $derived(g.scope === 'spread' ? null : g.scope)

  const name = $derived(galleryName(g.spread))
  const title = $derived(
    !c
      ? `${name}: подбираем раскладки…`
      : `${name}${page ? (page === 'left' ? ', левая' : ', правая') : ''}: ${k} ${plural(k, ['раскладка', 'раскладки', 'раскладок'])}`,
  )

  // Почему ряд с перестановкой пуст — причину сообщает Go.
  const why = $derived(
    rows?.block === 'same'
      ? 'Перестановка возможна, только если есть и вертикальные, и горизонтальные фото'
      : rows?.block === 'many'
        ? page
          ? 'Перестановка доступна для страниц до 5 фото'
          : 'Перестановка доступна для разворотов до 7 фото'
        : 'Перестановка не даёт лучших раскладок',
  )

  // «Ещё» (027): Go может досчитать ряд только у разворота целиком (17+ фото).
  const canMore = $derived(g.scope === 'spread' && !!rows?.more)
  const lines = $derived(galleryLines(g.scope, { inOrder, rearranged, extra }, { why, canMore }))
  // Фильтр семейств — только у разворота и когда есть из чего выбирать.
  const families = $derived(g.scope === 'spread' ? familiesIn([inOrder, rearranged, extra]) : [])
  const fam = $derived(activeFamily(gallery.family, families))
  const famItems = $derived<{ id: FamilyFilter; label: string }[]>([{ id: 'all', label: 'Все' }, ...families])
  const orders = $derived(galleryOrders(lines, fam))
  const row = $derived(galleryRow(lines, gallery.order, { family: fam, current: sp?.layout, openTwins: gallery.openTwins }))
  const tiles = $derived(row.parts.reduce((n, p) => n + p.shown.length + (p.hidden > 0 ? 1 : 0) + (p.more ? 1 : 0), 0))

  // Плитка «Ещё»: ряд дописывается, первая новая карточка встаёт в начало видимой части.
  async function more(e: MouseEvent) {
    if (!gallery.cur) return
    // Карточек в DOM до «Ещё» — столько, сколько показано в части «Авто» (похожие свёрнуты).
    const have = row.parts.find((p) => p.line === ROW.inOrder)?.shown.length ?? inOrder.length
    const sc = (e.currentTarget as HTMLElement).closest<HTMLElement>('.scroll')!
    await gallery.loadMore()
    await tick()
    const first = sc.querySelectorAll<HTMLElement>('.card:not(.more)')[have]
    if (first) sc.scrollTo({ left: first.offsetLeft - 8, behavior: 'smooth' })
  }

  // Галерея закрывается с анимацией: в это время компонент inert, его $derived (sp) читать нельзя (derived_inert).
  function pick(ch: Choice) {
    if (!gallery.cur) return
    if (ch.key === sp?.layout) gallery.close()
    else gallery.setLayout(ch.key)
  }
</script>

<GalleryHead tabs={galleryTabs()} {title}>
  {#snippet filters()}
    {#if famItems.length > 2}
      <Seg items={famItems} value={fam} onpick={(f) => gallery.setFamily(f)} label="Семейство раскладок" />
    {/if}
  {/snippet}
</GalleryHead>

<TplRow count={tiles} empty={c ? row.empty : ''} watch={[g.scope, c?.rev, sp?.layout, gallery.order, fam]}>
  {#snippet side()}
    <SideList items={orders} value={gallery.order} onpick={(o) => gallery.setOrder(o)} label="Какой ряд показать" />
  {/snippet}
  {#each row.parts as p (p.line)}
    {#if p.sep}<TplSep label={p.sep} title="Другие варианты: воздух и с главным фото" />{/if}
    {#each p.shown as ch (ch.key)}
      <button
        class="card"
        class:current={ch.key === sp?.layout}
        style:width="{cardW}px"
        style:height="{H}px"
        onmouseenter={() => gallery.previewChoice(ch)}
        onmouseleave={() => gallery.previewChoice(null)}
        onclick={() => pick(ch)}
      >
        {#each ch.rects as r, i (i)}
          <span
            class="cell"
            style:left="{(r.x / sheet.w) * 100}%"
            style:top="{(r.y / sheet.h) * 100}%"
            style:width="{(r.w / sheet.w) * 100}%"
            style:height="{(r.h / sheet.h) * 100}%"
          ></span>
        {/each}
        <span class="spine"></span>
        {#if page}
          <span class="other" style:left={page === 'left' ? '50%' : '0'}></span>
        {/if}
      </button>
    {/each}
    {#if p.hidden > 0}
      {@const open = gallery.openTwins.includes(p.line)}
      <button
        class="card more"
        style:width="{cardW}px"
        style:height="{H}px"
        aria-expanded={open}
        onclick={() => gallery.toggleTwins(p.line)}
        title="Раскладки, почти не отличающиеся от соседних">{twinsLabel(p.hidden, open)}</button
      >
    {/if}
    {#if p.more}
      <button
        class="card more"
        style:width="{cardW}px"
        style:height="{H}px"
        disabled={gallery.moreBusy}
        onclick={more}
        title="Досчитать ещё 12 раскладок">{gallery.moreBusy ? 'Считаю…' : 'Ещё'}</button
      >
    {/if}
  {/each}
</TplRow>

<style>
  .card {
    all: unset;
    flex: none;
    position: relative;
    background: #fff;
    cursor: pointer;
    outline: 2px solid transparent;
    outline-offset: 2px;
    transition: outline-color 0.15s;
  }
  .card:hover {
    outline-color: #8a8a90;
  }
  .card.current {
    outline-color: var(--accent);
  }
  /* Плитка «Ещё» и «Похожие» — не раскладка: пунктир на фоне панели */
  .card.more {
    display: grid;
    place-items: center;
    background: transparent;
    outline: 1px dashed var(--line);
    outline-offset: 0;
    color: var(--muted);
    font-size: 13px;
    text-align: center;
  }
  .card.more:hover:not(:disabled) {
    outline-color: #8a8a90;
    color: var(--text);
  }
  .card.more:disabled {
    cursor: progress;
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
  /* Галерея страницы: чужая половина не участвует */
  .other {
    position: absolute;
    top: 0;
    bottom: 0;
    width: 50%;
    background: #555;
  }
</style>
