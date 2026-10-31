<script lang="ts">
  import { tick } from 'svelte'
  import { store } from '../../base/core/store.svelte'
  import { gallery } from './gallery.svelte'
  import { plural } from '../../base/words/plural'
  import { newSheetSize, sheetSize } from '../../base/geom/sheet'
  import LayoutCard from '../../base/tpl/LayoutCard.svelte'
  import MoreTile from '../../base/tpl/MoreTile.svelte'
  import Seg from '../../base/tpl/Seg.svelte'
  import SideList from '../../base/tpl/SideList.svelte'
  import TplRow from '../../base/tpl/TplRow.svelte'
  import TplSep from '../../base/tpl/TplSep.svelte'
  import GalleryHead, { galleryName, galleryTabs } from './GalleryHead.svelte'
  import { foldLink, foldNote, foldNoteShown } from './foldnote'
  import type { Choice, ChoiceRows, LayoutScope } from '../../base/core/types'
  import {
    activeFamily,
    familiesIn,
    galleryCount,
    galleryLines,
    galleryOrders,
    galleryRow,
    ROW,
    tileTitle,
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
  // 105: раскладки «Разброса» с фото на сгибе, скрытые правилом «Только широкие»; считает Go, у страницы их нет.
  const fold = $derived(g.scope === 'spread' ? foldNote(sp?.foldHidden) : null)

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

  // 105-qa-3: «Другие ▸» в шапке — когда на экране есть черта «другие» (не первая часть ряда). Прокручивает ряд к ней.
  const hasOthers = $derived(row.parts.some((p) => p.line === ROW.extra && !!p.sep))
  function toOthers(e: MouseEvent) {
    const root = (e.currentTarget as HTMLElement).closest('header')?.parentElement
    const sep = root?.querySelector<HTMLElement>('[data-sep="extra"]')
    const sc = sep?.closest<HTMLElement>('.scroll')
    if (sep && sc) sc.scrollTo({ left: sep.offsetLeft - 8, behavior: 'smooth' })
  }

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
    {#if hasOthers}
      <button type="button" class="others" title="Прокрутить ряд к «Другим вариантам»" onclick={toOthers}>Другие ▸</button>
    {/if}
  {/snippet}
</GalleryHead>

<TplRow count={tiles} empty={c ? row.empty : ''} watch={[g.scope, c?.rev, sp?.layout, gallery.order, fam]}>
  {#snippet side()}
    <SideList items={orders} value={gallery.order} onpick={(o) => gallery.setOrder(o)} label="Какой ряд показать" />
  {/snippet}
  {#each row.parts as p (p.line)}
    {#if p.sep}<TplSep key="extra" label={p.sep} title="Другие варианты: воздух и с главным фото" />{/if}
    {#if fold && p.line === ROW.extra && foldNoteShown(fam)}
      <p class="foldnote">
        <span>{fold.text}<button type="button" class="link" onclick={foldLink.open}>{fold.link}</button></span>
      </p>
    {/if}
    {#each p.shown as ch (ch.key)}
      <LayoutCard
        rects={ch.rects}
        {sheet}
        h={H}
        {page}
        current={ch.key === sp?.layout}
        title={tileTitle(ch)}
        onmouseenter={() => gallery.previewChoice(ch)}
        onmouseleave={() => gallery.previewChoice(null)}
        onclick={() => pick(ch)}
      />
    {/each}
    {#if p.hidden > 0}
      {@const open = gallery.openTwins.includes(p.line)}
      <MoreTile
        {sheet}
        h={H}
        expanded={open}
        label={twinsLabel(p.hidden, open)}
        title="Раскладки, почти не отличающиеся от соседних"
        onclick={() => gallery.toggleTwins(p.line)}
      />
    {/if}
    {#if p.more}
      <MoreTile
        {sheet}
        h={H}
        disabled={gallery.moreBusy}
        label={gallery.moreBusy ? 'Считаю…' : 'Ещё'}
        title="Досчитать ещё 12 раскладок"
        onclick={more}
      />
    {/if}
  {/each}
</TplRow>

<style>
  .others {
    flex: none;
    height: 26px;
    padding: 0 10px;
    font-size: 12px;
  }
  /* Серая заметка в начале «Других вариантов», как пустой ряд (TplRow .empty): высота плитки, текст по центру */
  .foldnote {
    flex: none;
    box-sizing: border-box;
    width: 230px;
    height: 80px;
    margin: 0;
    display: grid;
    align-content: center;
    color: var(--muted);
    font-size: 12px;
    line-height: 1.35;
  }
  /* Ссылка — как «Готовые развороты» у пустого листа: серая, подчёркнутая */
  .link {
    all: unset;
    cursor: pointer;
    text-decoration: underline;
  }
  .link:hover {
    color: var(--text);
  }
  .link:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: 2px;
  }
</style>
