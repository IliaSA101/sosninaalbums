<script lang="ts" module>
  export type PhotoColors = { state: 'load' | 'ok' | 'err'; list: string[] }
</script>

<script lang="ts">
  import { tick } from 'svelte'
  import ColorChip from './ColorChip.svelte'
  import type { ColorWords } from './colorwords'
  import { nameOf, PHOTO_TAB, SETS } from './palette'

  // Шапка окна (имя и HEX цвета), вкладки наборов и 8 образцов выбранной вкладки (макет варианта 1, 109-5).
  // «С фото» — шестая вкладка, цвета приходят от Go; без листа (sheet не передан) её нет.
  let {
    shown,
    cur,
    tab,
    withPhoto,
    photo,
    words,
    ontab,
    onpick,
    onpreview,
    inheritOn = false,
  }: {
    shown: string // что показано в шапке: цвет под мышью или текущий
    cur: string
    tab: string
    withPhoto: boolean
    photo: PhotoColors
    words: ColorWords // «На развороте» / «На листе» / «На обложке»
    ontab: (id: string) => void
    onpick: (hex: string) => void
    onpreview: (hex: string | null) => void
    inheritOn?: boolean // свой цвет не задан: текущий цвет — цвет альбома, шапка так и зовёт его (115)
  } = $props()

  const tabs = $derived([...SETS.map((s) => ({ id: s.id, title: s.title })), ...(withPhoto ? [{ id: PHOTO_TAB, title: 'С фото' }] : [])])
  const set = $derived(SETS.find((s) => s.id === tab))
  let strip = $state<HTMLElement>()

  // ← → переходят по вкладкам по кругу; Home/End — на края (фокус идёт за выбором).
  async function onkey(e: KeyboardEvent) {
    const i = tabs.findIndex((t) => t.id === tab)
    const to =
      e.key === 'ArrowRight' ? i + 1 : e.key === 'ArrowLeft' ? i - 1 : e.key === 'Home' ? 0 : e.key === 'End' ? tabs.length - 1 : null
    if (to === null) return
    e.preventDefault()
    ontab(tabs[(to + tabs.length) % tabs.length].id)
    await tick()
    strip?.querySelector<HTMLElement>('[aria-selected="true"]')?.focus()
  }
</script>

<div class="head">
  <div class="cur" style:background={shown || 'transparent'}></div>
  <div class="name">
    <b>{shown ? (inheritOn && shown === cur ? 'Как в альбоме' : nameOf(shown)) : 'Цвет не задан'}</b>
    <span>{shown}</span>
  </div>
</div>

<div class="tabs" role="tablist" aria-label="Наборы цветов" tabindex="-1" bind:this={strip} onkeydown={onkey}>
  {#each tabs as t (t.id)}
    <button
      type="button"
      role="tab"
      class:on={t.id === tab}
      aria-selected={t.id === tab}
      tabindex={t.id === tab ? 0 : -1}
      onclick={() => ontab(t.id)}>{t.title}</button
    >
  {/each}
</div>

<div class="grid" role="tabpanel" aria-label={tabs.find((t) => t.id === tab)?.title}>
  {#if set}
    {#each set.colors as c (c.hex)}
      <ColorChip hex={c.hex} size={26} on={c.hex === cur} {onpick} {onpreview} />
    {/each}
  {:else if photo.state === 'ok' && photo.list.length > 0}
    {#each photo.list as hex (hex)}
      <ColorChip {hex} size={26} photo on={hex === cur} {onpick} {onpreview} />
    {/each}
  {:else}
    <span class="none">
      {photo.state === 'load' ? 'Подбираем цвета…' : photo.state === 'err' ? 'Не получилось подобрать цвета' : `${words.onCap} нет фото`}
    </span>
  {/if}
</div>

<style>
  .head {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-bottom: 8px;
  }
  .cur {
    flex: none;
    width: 34px;
    height: 24px;
    border-radius: 5px;
    border: 1px solid var(--line);
  }
  .name {
    min-width: 0;
    line-height: 1.2;
  }
  .name b {
    font-weight: 600;
    font-size: 12px;
  }
  .name span {
    display: block;
    min-height: 1.2em;
    color: var(--muted);
    font-family: Consolas, monospace;
    font-size: 11px;
  }
  .tabs {
    display: flex;
    flex-wrap: wrap;
    gap: 4px;
    margin-bottom: 8px;
  }
  .tabs button {
    height: 24px;
    padding: 0 6px; /* при 8 px вторая строка выходила за 250 px, и вкладки шли в 3 строки */
    font-size: 12px;
    background: #333336;
  }
  .tabs button.on {
    background: #4a4a50;
    color: #fff;
    box-shadow: inset 0 -2px 0 var(--accent);
  }
  .grid {
    display: flex;
    flex-wrap: wrap;
    gap: 5px;
    min-height: 26px;
  }
  .none {
    color: var(--muted);
    line-height: 26px;
  }
</style>
