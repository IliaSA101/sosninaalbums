<script lang="ts">
  import { store } from '../../base/core/store.svelte'
  import { prefs } from '../../base/prefs/prefs.svelte'
  import { keymap } from '../../base/keys/keymap.svelte'
  import Icon from '../../base/ui/Icon.svelte'
  import { ownMarks } from './ownmarks.svelte'
  import { flagClick, hasOwn, LABEL_VIEW, ratingClick, sharedMark, starsLit } from './markbar'

  // Полоска выделенных фото в панели ленты (спеки 004 п. 7, 024): «N ×» всегда, отметки — по уровню.
  // Каждый клик — одна правка (один шаг отмены на все выделенные). Корень не зависит от родителя:
  // живёт и в строке панели, и в окне «⋯».
  const shared = $derived(
    sharedMark(
      store.selectedIds.flatMap((id) => store.byId.get(id) ?? []),
      store.marks,
    ),
  )
  const n = $derived(store.selectedIds.length)

  // Звезда под курсором (или в фокусе): горят звёзды до неё, как в Lightroom.
  let hover = $state<number | null>(null)
  const lit = $derived(starsLit(hover, shared.rating))

  // Кнопки внутри ленты гасят pointerdown: иначе он начнёт перенос или снимет выделение.
  const stop = (e: PointerEvent) => e.stopPropagation()
  // Фокус остаётся где был: иначе Enter (хоткей «добавить выделенные») нажмёт кнопку повторно и снимет отметку.
  const keep = (e: MouseEvent) => e.preventDefault()
</script>

<div class="markbar" role="toolbar" aria-label="Выделенные фото" data-guide="mark-bar">
  <button
    class="ghost clear"
    title="Выделено: {n}. Снять выделение (Esc)"
    aria-label="Снять выделение ({n} фото)"
    onpointerdown={stop}
    onmousedown={keep}
    onclick={() => store.selected.clear()}>{n}<Icon name="x" size={12} /></button
  >

  {#if prefs.can('marks')}
    <span class="sepv"></span>

    <div class="group" class:hov={hover !== null} role="group" aria-label="Оценка" onpointerleave={() => (hover = null)}>
      {#each [1, 2, 3, 4, 5] as s (s)}
        <button
          class="ico star"
          class:on={s <= lit}
          title={shared.rating === s ? 'Снять оценку' : `Оценка ${s}`}
          aria-label={`Оценка ${s}`}
          aria-pressed={shared.rating === s}
          onpointerdown={stop}
          onmousedown={keep}
          onpointerenter={() => (hover = s)}
          onfocus={() => (hover = s)}
          onblur={() => (hover = null)}
          onclick={() => ownMarks.setMarks(store, { rating: ratingClick(s, shared.rating) })}>★</button
        >
      {/each}
    </div>

    <div class="group" role="group" aria-label="Цветная метка">
      {#each LABEL_VIEW as c (c.id)}
        <button
          class="ico dot"
          class:on={shared.label === c.id}
          style:--c="var(--lbl-{c.id})"
          title={shared.label === c.id ? 'Снять метку' : `Метка: ${c.name}`}
          aria-label={`Метка: ${c.name}`}
          aria-pressed={shared.label === c.id}
          onpointerdown={stop}
          onmousedown={keep}
          onclick={() => ownMarks.setMarks(store, { label: c.id, toggle: true })}
        ></button>
      {/each}
    </div>

    <div class="group flags" role="group" aria-label="Флажок">
      <button
        class="flag"
        class:on={shared.flag === 1}
        aria-pressed={shared.flag === 1}
        title={shared.flag === 1 ? keymap.tip('Снять флажок', 'markUnflag') : keymap.tip('Отобрано', 'markPick')}
        aria-label="Отобрано"
        onpointerdown={stop}
        onmousedown={keep}
        onclick={() => ownMarks.setMarks(store, { flag: flagClick(1, shared.flag) })}><Icon name="flag" size={16} /></button
      >
      <button
        class="flag"
        class:on={shared.flag === -1}
        aria-pressed={shared.flag === -1}
        title={shared.flag === -1 ? keymap.tip('Снять флажок', 'markUnflag') : keymap.tip('Отклонено', 'markReject')}
        aria-label="Отклонено"
        onpointerdown={stop}
        onmousedown={keep}
        onclick={() => ownMarks.setMarks(store, { flag: flagClick(-1, shared.flag) })}><Icon name="flag-off" size={16} /></button
      >
    </div>

    <button
      class="ghost flag"
      title="Как в Lightroom: убрать свои отметки выделенных — снова действуют отметки из файлов"
      aria-label="Как в Lightroom"
      disabled={!hasOwn(store.selectedIds, store.marks)}
      onpointerdown={stop}
      onmousedown={keep}
      onclick={() => ownMarks.resetMarks(store)}><Icon name="revert" size={16} /></button
    >
  {/if}
</div>

<style>
  .markbar {
    display: inline-flex;
    align-items: center;
    flex-wrap: nowrap;
    white-space: nowrap;
    gap: 6px;
    max-height: 30px;
  }
  .markbar button {
    height: 26px;
    font-size: 12px;
  }
  .clear {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    padding: 0 6px;
  }
  .sepv {
    width: 1px;
    height: 18px;
    background: var(--line);
  }
  .group {
    display: inline-flex;
    flex-wrap: nowrap;
  }
  /* Две кнопки флажков по 26 px вплотную сливались в одну — разводим. */
  .flags {
    gap: 4px;
  }
  .ico {
    width: 16px;
    padding: 0;
    border-radius: 3px;
    background: transparent;
    color: var(--muted);
  }
  .ico:hover:not(:disabled) {
    background: transparent;
  }
  .star {
    font-size: 14px;
    line-height: 1;
  }
  .star.on {
    color: var(--accent);
  }
  /* При наведении подсвечены звёзды до курсора — чуть бледнее, чтобы отличить от поставленной оценки */
  .hov .star.on {
    color: color-mix(in srgb, var(--accent) 70%, transparent);
  }
  /* Кружок метки: кольцо у текущей, у остальных — просто цвет */
  .dot {
    position: relative;
  }
  .dot::before {
    content: '';
    position: absolute;
    inset: 8px 3px;
    border-radius: 50%;
    background: var(--c);
    opacity: 0.85;
  }
  .dot:hover::before {
    opacity: 1;
  }
  .dot.on {
    border-color: var(--text);
  }
  .dot.on::before {
    opacity: 1;
    box-shadow: 0 0 0 1.5px var(--text);
  }
  .flag {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 26px;
    padding: 0;
  }
  .flag.on {
    background: #48484d;
    border-color: var(--accent);
  }
</style>
