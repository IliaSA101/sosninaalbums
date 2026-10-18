<script lang="ts">
  import type { Family } from '../../base/core/types'
  import type { Unit } from '../../base/geom/sheet'
  import LayoutCard from '../../base/tpl/LayoutCard.svelte'
  import MoreTile from '../../base/tpl/MoreTile.svelte'
  import Seg from '../../base/tpl/Seg.svelte'
  import SideList, { type SideItem } from '../../base/tpl/SideList.svelte'
  import TplItem from '../../base/tpl/TplItem.svelte'
  import TplRow from '../../base/tpl/TplRow.svelte'
  import TplSep from '../../base/tpl/TplSep.svelte'
  import { FAMILIES, tileTitle, twinsLabel } from '../../panels/gallery/variants'
  import type { AutoDialog, RowMode } from './autobuild.svelte'
  import { VARIETY_OPTIONS } from './autobuild'
  import { autoRows, fallbackNote, familiesHint, layoutsStatus, layoutsWaiting, shownFamilies, spreadTitle, twinKey } from './autolayouts'

  // Блок «Раскладки» окна автосборки (065): фильтр семейств, переключатель ряда слева и ряд плиток — то же устройство, что у полосы
  // «Варианты» над разворотом (LayoutCard, TplRow, SideList). Щелчок по плитке ничего не ставит: это предпросмотр.
  // «Разнообразие раскладок» живёт здесь же — оно меняет то, что видно в ряду.
  let { ui, famOn, unit, empty }: { ui: AutoDialog; famOn: boolean; unit: Unit; empty: boolean } = $props()

  const H = 80 // высота плитки, как в полосе «Варианты»
  const pv = $derived(ui.preview)
  const sheet = $derived({ w: pv?.w || 1, h: pv?.h || 1 })

  // «Все» + семейства «Вариантов» окна и выбранные (выбор не прячется)
  const shown = $derived(shownFamilies(pv, ui.families))
  const famItems = $derived<{ id: Family | 'all'; label: string }[]>([
    { id: 'all', label: 'Все' },
    ...FAMILIES.filter((f) => shown.includes(f.id)),
  ])
  const famValues = $derived<(Family | 'all')[]>(ui.families.length > 0 ? ui.families : ['all'])

  // Раскрытые «Похожие» — память блока: ключ «число фото|линия»
  let open = $state<string[]>([])
  const toggleTwins = (k: string) => (open = open.includes(k) ? open.filter((x) => x !== k) : [...open, k])
  const rows = $derived(pv ? autoRows(pv, ui.families, open) : null)

  const orders = $derived<SideItem<RowMode>[]>([
    {
      id: 'samples',
      label: 'Варианты',
      count: rows?.count,
      title: 'Возможные варианты раскладки для каждого числа фото в сборке — как в полосе «Варианты»',
    },
    { id: 'build', label: 'Что соберётся', count: pv?.spreads.length, title: `Будущие ${unit.many} по порядку` },
  ])
  const tiles = $derived(!pv ? 0 : ui.rowMode === 'samples' ? (rows?.parts.length ?? 0) : pv.spreads.length)

  const waiting = $derived(layoutsWaiting(ui))
  const note = $derived(layoutsStatus({ error: ui.previewError, empty, waiting }))
  const noneText = $derived(ui.previewError ? 'Не удалось показать раскладки' : empty ? 'Нет фото для сборки' : '')
  const variety = $derived(VARIETY_OPTIONS.find((o) => o.v === ui.variety))
</script>

<div class="lay">
  <div class="more">
    Раскладки
    {#if famOn}
      <Seg items={famItems} values={famValues} onpick={(f) => ui.setFamilies(f)} label="Семейства раскладок" />
    {/if}
  </div>

  <TplRow count={tiles} empty={noneText} dim={waiting && tiles > 0} watch={[ui.rowMode, ui.families.join(','), ui.variety]}>
    {#snippet side()}
      <SideList items={orders} value={ui.rowMode} onpick={(m) => ui.setRowMode(m)} label="Какой ряд показать" />
    {/snippet}
    {#if pv && ui.rowMode === 'samples' && rows}
      {#each rows.parts as part (part.photos)}
        <TplSep label="{part.photos} фото" />
        {#if part.row.parts.length === 0}
          <div class="none">{ui.families.length > 0 ? 'нет — будет лучшая' : part.row.empty}</div>
        {/if}
        {#each part.row.parts as p (p.line)}
          {#if p.sep}<TplSep label={p.sep} title="Другие варианты: воздух и с главным фото" />{/if}
          {#each p.shown as ch (ch.key)}
            <LayoutCard rects={ch.rects} {sheet} h={H} title={tileTitle(ch)} />
          {/each}
          {#if p.hidden > 0}
            {@const k = twinKey(part.photos, p.line)}
            {@const isOpen = open.includes(k)}
            <MoreTile
              {sheet}
              h={H}
              expanded={isOpen}
              label={twinsLabel(p.hidden, isOpen)}
              title="Раскладки, почти не отличающиеся от соседних"
              onclick={() => toggleTwins(k)}
            />
          {/if}
        {/each}
      {/each}
    {:else if pv}
      {#each pv.spreads as s, i (i)}
        <TplItem name={String(i + 1)} note="{s.photos} фото">
          <LayoutCard rects={s.rects} {sheet} h={H} fallback={s.fallback} title={spreadTitle(s)} />
        </TplItem>
      {/each}
    {/if}
  </TplRow>

  <p class="status" role="status" class:err={ui.previewError}>
    {#if note}
      {note}
    {:else}
      {familiesHint(ui.families)}
      {#if fallbackNote(pv?.fallback ?? 0, unit)}
        · <span class="warn">{fallbackNote(pv?.fallback ?? 0, unit)}</span>
      {/if}
    {/if}
  </p>

  <!-- 041: выбор из трёх — сегменты, как «Разворот / Левая / Правая» в галерее -->
  <div class="more">
    Разнообразие раскладок
    <Seg
      items={VARIETY_OPTIONS.map((o) => ({ id: o.v, label: o.label }))}
      value={ui.variety}
      onpick={(v) => ui.setVariety(v)}
      label="Разнообразие раскладок"
    />
  </div>
  <p class="hint" role="status">{variety?.hint}</p>
</div>

<style>
  .lay {
    display: flex;
    flex-direction: column;
    gap: 4px;
    margin-top: 4px;
    padding-top: 10px;
    border-top: 1px solid var(--line);
  }
  .more {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 8px;
  }
  .status {
    margin: 0;
    min-height: 18px;
    color: var(--muted);
    font-size: 12px;
  }
  .status.err {
    color: #e5645a;
  }
  .warn {
    color: var(--text);
  }
  .hint {
    margin: 0;
    min-height: 16px;
    color: var(--muted);
    font-size: 12px;
  }
  /* У числа фото нет раскладок выбранных семейств: сборка возьмёт лучшую */
  .none {
    flex: none;
    padding: 30px 8px 0 0;
    color: var(--muted);
    font-size: 13px;
  }
</style>
