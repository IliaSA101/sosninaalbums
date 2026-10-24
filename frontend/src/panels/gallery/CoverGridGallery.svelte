<script lang="ts">
  import { store, Api } from '../../base/core/store.svelte'
  import { prefs } from '../../base/prefs/prefs.svelte'
  import { gallery } from './gallery.svelte'
  import { errText } from '../../base/words/errtext'
  import { tplio } from '../../shell/menu/tplio.svelte'
  import { applyCoverChoice } from '../layouts/gridactions'
  import { coverPreview, coverSourceTexts, dropTitle, N_ITEMS } from './gridgallery'
  import type { CoverGridChoices, GridChoice, LayoutScope } from '../../base/core/types'
  import TplRow from '../../base/tpl/TplRow.svelte'
  import TplItem from '../../base/tpl/TplItem.svelte'
  import TplSep from '../../base/tpl/TplSep.svelte'
  import SideList from '../../base/tpl/SideList.svelte'
  import Seg from '../../base/tpl/Seg.svelte'
  import { openN } from '../layouts/coverbar'
  import { tplSources } from '../../base/tpl/tplsource'
  import GalleryHead from './GalleryHead.svelte'
  import { liveRadius, shapeCss } from '../../base/geom/shape'

  // Галерея раскладок сторонки обложки (127): та же полоса «Шаблоны», что у разворота (0.26-127-13), но без вкладок
  // и «Сохранить как шаблон…». Ряды «По фото · Встроенные · Мои» и доли зоны приходят от Go (CoverGridChoices);
  // здесь только перевод долей в проценты карточки и в мм листа для предпросмотра.
  const H = 80 // высота карточки
  // g приходит от LayoutGallery: пока галерея уезжает, gallery.cur уже null.
  let { g }: { g: { spread: string; scope: LayoutScope } } = $props()
  const scope = $derived(g.scope)
  const cs = $derived(store.cover?.sides?.find((x) => x.scope === scope))
  const sheet = $derived(store.cover?.sheet)
  // «Мои» — шаблоны (уровень «Шаблоны»), остальное — «Варианты» (0.26-127-14).
  const canVar = $derived(prefs.can('variants'))
  const canTpl = $derived(prefs.can('templates'))

  // Фильтр: пока не выбран руками — по местам сторонки (0 — «1», 7+ — «7»), как при ▦ по ‹ ›.
  let nPick = $state<number | null>(null)
  const nWant = $derived(nPick ?? (cs ? openN(cs) : 1))
  $effect(() => {
    void scope // другая сторонка — снова фильтр по умолчанию
    nPick = null
  })

  // Ответ Go; ключ — для какой сторонки и вкладки он посчитан.
  let c = $state<{ key: string; res: CoverGridChoices } | null>(null)
  let loading = $state(false)
  let req = 0
  const key = $derived(`${scope}|${nWant}`)
  $effect(() => {
    const k = key
    const sc = scope
    const n = nWant
    void [store.view.rev, tplio.seq] // правка вида или шаблоны из файла — перезапрос
    const my = ++req
    loading = true
    Api.CoverGridChoices(sc, n)
      .then((res) => {
        if (my === req) c = { key: k, res: res as unknown as CoverGridChoices }
      })
      .catch((e) => {
        if (my === req) store.notify(errText(e), 'error')
      })
      .finally(() => {
        if (my === req) loading = false
      })
  })
  const res = $derived(c && c.key === key ? c.res : null)
  const box = $derived(res?.box ?? cs?.box)
  // Плитка — в пропорциях зоны сторонки.
  const cardW = $derived(box && box.h > 0 ? (H * box.w) / box.h : H)

  const texts = $derived(coverSourceTexts(cs?.places ?? 0, nWant))
  // Ряды по уровням: без «Вариантов» нет «По фото» и «Встроенных», без «Шаблонов» — «Мои».
  const build = (r: CoverGridChoices | null) =>
    tplSources(
      canVar ? (r?.builtin ?? []) : [],
      canTpl ? (r?.mine ?? []) : [],
      gallery.source,
      texts,
      canVar ? (r?.byPhoto ?? []) : undefined,
    )
  const view = $derived(build(res))
  // Пока ответ на новую вкладку не пришёл, пункты считаются по прежнему ответу: иначе на миг серые с ложной причиной.
  // Первая загрузка: пункты без чисел и не серые.
  const shown = $derived(res ? view.items : c ? build(c.res).items : view.items.map(({ id, label }) => ({ id, label })))
  const sideItems = $derived(shown.filter((i) => (i.id === 'mine' ? canTpl : i.id === 'all' || canVar)))
  const tiles = $derived(view.parts.reduce((n, p) => n + p.list.length, 0))
  const title = $derived(loading ? 'Считаю…' : (cs?.label ?? ''))

  // Пока галерея уезжает (slide), компонент уже inert: читать его $derived в обработчике нельзя (derived_inert),
  // а повторный щелчок применил бы плитку дважды. Закрыта — обработчики ничего не читают.
  function hover(ch: GridChoice | null) {
    if (!gallery.cur) return
    gallery.previewChoice(ch && box && sheet ? { key: ch.id, rects: coverPreview(ch, box, sheet) } : null)
  }
  function pick(ch: GridChoice) {
    if (!gallery.cur) return
    const sc = scope
    gallery.close()
    void applyCoverChoice(sc, ch)
  }
</script>

<GalleryHead tabs={false} {title}>
  {#snippet filters()}
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
      <TplItem>
        <button
          class="card"
          class:dim={loading}
          style:width="{cardW}px"
          style:height="{H}px"
          onmouseenter={() => hover(ch)}
          onmouseleave={() => hover(null)}
          onclick={() => pick(ch)}
        >
          {#each ch.boxes as r, i (i)}
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
          {#if ch.drop > 0}<span class="drop" title={dropTitle(ch.drop)}>−{ch.drop}</span>{/if}
        </button>
      </TplItem>
    {/each}
  {/each}
</TplRow>

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
