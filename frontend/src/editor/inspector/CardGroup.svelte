<script lang="ts">
  import type { Snippet } from 'svelte'
  import { drag } from '../../base/drag/drag.svelte'
  import { pickDrop } from '../../base/drag/hit'
  import { cardGroups, dropLanded, useCard, type GroupId } from '../../base/prefs/cardgroups.svelte'
  import { prefs } from '../../base/prefs/prefs.svelte'
  import Icon from '../../base/ui/Icon.svelte'
  import MenuList from '../../base/ui/MenuList.svelte'
  import type { MenuNode } from '../../base/ui/menulist'
  import { tipped } from '../../base/ui/tip.svelte'
  import { spot } from '../../shell/guide/spotstate.svelte'
  import { MoreList } from '../places/ministate.svelte'
  import { dropOpen as dragOpens, groupShown } from './groupsum'

  // Сворачиваемая группа карточки (спека 124 §4): тонкий заголовок — шеврон, название, значение в свёрнутом виде, «⋯».
  // Образец — Section панели деталей: аккордеон grid 0fr → 1fr, тело не размонтируется (поля не теряют набранное,
  // якоря справочника есть и в свёрнутом теле), inert прячет его от Tab. Память — cardGroups, отдельно для панели
  // деталей и плавающей карточки (хост приходит из контекста карточки). Группа без строк на уровне не рисуется.
  // dropOpen — у группы с зоной «Фото фона»: пока тянут фото, она раскрыта без записи, а удачный бросок в её зону
  // оставляет её открытой и запоминает (0.26-124-23), иначе новая строка фона спряталась бы вместе с «Править».
  let {
    id,
    title,
    summary = '',
    dropOpen = false,
    more,
    onmore,
    moreGuide,
    children,
  }: {
    id: GroupId
    title: string
    summary?: string
    dropOpen?: boolean
    more?: MenuNode[]
    onmore?: (n: MenuNode) => void
    moreGuide?: string // запасной якорь справочника: кнопка «⋯» группы (data-guide-hold)
    children: Snippet
  } = $props()

  const card = useCard()
  const host = $derived(card.host)
  const open = $derived(cardGroups.isOpen(host, id) || (dropOpen && dragOpens(drag)))
  const bodyId = $derived(`group-${host}-${id.replace('.', '-')}`)
  let root = $state<HTMLElement>()

  function toggle() {
    const now = cardGroups.toggle(host, id)
    // Кольцо справочника или тура внутри свёрнутой руками группы осталось бы висеть над пустым местом.
    if (!now && spot.cur && root?.contains(spot.cur.el)) spot.cancel()
  }

  // Цель переноса — зона фона внутри этой группы (а не рамка фона на холсте с теми же spread и scope): запоминаем
  // сам элемент под курсором и сверяем с корнем группы, до отпускания.
  let aimed = false
  $effect(() => {
    if (!dropOpen || !drag.active) return
    const t = drag.target
    const el = t?.kind === 'backdrop' ? pickDrop(document.elementsFromPoint(drag.x, drag.y)) : null
    aimed = !!el && !!root?.contains(el)
  })
  // Go принял бросок (dropact → dropLanded): группа остаётся открытой и запоминается, как после «Показать».
  let landed = dropLanded.n
  $effect(() => {
    const n = dropLanded.n
    if (n === landed) return
    landed = n
    if (!aimed) return
    aimed = false
    cardGroups.open(host, id)
  })

  const list = new MoreList()
  const pick = (n: MenuNode) => {
    list.close()
    onmore?.(n)
  }
</script>

{#if groupShown(id, prefs.can)}
  <section class="grp" data-group={id} data-group-host={host} bind:this={root}>
    <div class="bar">
      <button type="button" class="head" aria-expanded={open} aria-controls={bodyId} onclick={toggle}>
        <svg class="chev" class:open viewBox="0 0 12 12" aria-hidden="true"><path d="M4 2.5 7.5 6 4 9.5" /></svg>
        <span class="ttl" {title}>{title}</span>
        {#if !open && summary}<span class="sum" title={summary}>{summary}</span>{/if}
      </button>
      {#if more && more.length > 0}
        <button
          type="button"
          class="more"
          class:on={list.open}
          aria-label="Ещё: {title}"
          aria-haspopup="menu"
          aria-expanded={list.open}
          data-guide-hold={moreGuide}
          use:tipped={{ text: list.open ? '' : 'Ещё', side: 'top' }}
          onclick={(e) => list.toggle(e.currentTarget, true)}><Icon name="more" size={14} /></button
        >
      {/if}
    </div>
    <div class="body" class:open id={bodyId} inert={!open}>
      <div class="inner">{@render children()}</div>
    </div>
  </section>
  {#if list.open && more && more.length > 0}
    <MenuList items={more} at={list.at} side="left" keep={list.btn} onpick={pick} onclose={() => list.close()} />
  {/if}
{/if}

<style>
  .grp {
    margin-top: 8px;
  }
  .bar {
    display: flex;
    align-items: center;
    gap: 2px;
    height: 22px;
  }
  .head {
    all: unset;
    box-sizing: border-box;
    flex: 1;
    min-width: 0;
    height: 22px;
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 12px;
    font-weight: 600;
    cursor: pointer;
  }
  .head:hover .ttl {
    color: #fff;
  }
  .head:focus-visible,
  .more:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: -2px;
  }
  .chev {
    width: 12px;
    height: 12px;
    flex: none;
    fill: none;
    stroke: var(--muted);
    stroke-width: 1.6;
    stroke-linecap: round;
    stroke-linejoin: round;
    transition: transform var(--move);
  }
  .chev.open {
    transform: rotate(90deg);
  }
  /* Длинное название («Размеры для типографии» в узкой карточке) сжимается с многоточием, полностью — в title */
  .ttl {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .sum {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-weight: 400;
    color: var(--muted);
  }
  .more {
    all: unset;
    box-sizing: border-box;
    flex: none;
    width: 22px;
    height: 22px;
    display: grid;
    place-items: center;
    border-radius: 4px;
    color: #cfcfd4;
    cursor: pointer;
  }
  .more:hover,
  .more.on {
    background: rgba(255, 255, 255, 0.08);
  }
  .body {
    display: grid;
    grid-template-rows: 0fr;
    transition: grid-template-rows var(--move);
  }
  .body.open {
    grid-template-rows: 1fr;
  }
  .inner {
    min-height: 0;
    overflow: hidden;
  }
  /* Первая строка группы встаёт вплотную к заголовку: свои отступы строк (margin-top 8–10) нужны только между строками */
  .inner > :global(:first-child) {
    margin-top: 0;
  }
</style>
