<script lang="ts">
  import { SHEET_PRESETS } from '../../base/geom/sheet'
  import { prefs } from '../../base/prefs/prefs.svelte'
  import { newDoc } from './newdoc.svelte'
  import { sizeHint } from './newproduct'

  // «Новое изделие» в стартовом окне (спека 031): альбом или виньетка — отдельный лист.
  // oncreated — проект заменён, окно можно закрыть.
  let { oncreated }: { oncreated: () => void } = $props()

  const hint = $derived(sizeHint(newDoc.pick, newDoc.w, newDoc.h))

  // Уровень снизили, пока был выбран лист, — возвращаемся к альбому.
  $effect(() => {
    if (!prefs.can('vignette') && newDoc.kind === 'sheet') newDoc.kind = 'album'
  })

  async function create() {
    if (newDoc.ok && (await newDoc.create())) oncreated()
  }
  // «Новый альбом» создаёт сразу, как в 0.9.1.
  async function newAlbum() {
    newDoc.kind = 'album'
    await create()
  }
</script>

<!-- Виньетка — с «Стандартного» (решение 80): на «Простом» кнопки нет -->
<!-- Ряд изделий как в 0.9.1: обычные кнопки; жёлтая в окне одна — «Продолжить». Выбранная виньетка — рамкой. -->
<div class="products">
  <button disabled={newDoc.busy} onclick={newAlbum}>Новый альбом</button>
  {#if prefs.can('vignette')}
    <button
      class:on={newDoc.kind === 'sheet'}
      aria-pressed={newDoc.kind === 'sheet'}
      title="Отдельный плакат класса 20×30 или 30×40 — без альбома"
      onclick={() => (newDoc.kind = newDoc.kind === 'sheet' ? 'album' : 'sheet')}>Виньетка — отдельный лист</button
    >
  {/if}
  <button disabled title="Скоро">Календарь — скоро</button>
</div>

{#if newDoc.kind === 'sheet'}
  <div class="sizes" role="radiogroup" aria-label="Размер листа">
    {#each SHEET_PRESETS as p, i (p.label)}
      <button class="chip" class:on={newDoc.pick === i} role="radio" aria-checked={newDoc.pick === i} onclick={() => newDoc.choose(i)}
        ><b>{p.label}</b><small>{p.w}×{p.h} мм</small></button
      >
    {/each}
    <button
      class="chip"
      class:on={newDoc.pick === 'custom'}
      role="radio"
      aria-checked={newDoc.pick === 'custom'}
      onclick={() => newDoc.choose('custom')}><b>Свой размер</b></button
    >
    {#if newDoc.pick === 'custom'}
      <span class="own">
        <input type="number" min="50" max="1000" aria-label="Ширина листа, мм" bind:value={newDoc.w} />
        ×
        <input type="number" min="50" max="1000" aria-label="Высота листа, мм" bind:value={newDoc.h} />
        мм
      </span>
    {/if}
  </div>
  <p class="note" class:bad={hint !== ''}>
    {hint || 'Один лист без корешка; в экспорте — файл на лист (01.jpg…). Листов можно добавить сколько нужно.'}
  </p>
{/if}

{#if newDoc.kind === 'sheet'}
  <button class="make" disabled={!newDoc.ok || newDoc.busy} onclick={create}>Создать лист</button>
{/if}

<style>
  .products {
    display: flex;
    gap: 8px;
  }
  .products button {
    flex: 1;
  }
  .products button:disabled {
    opacity: 0.45;
  }
  /* Выбор — рамкой, не заливкой: жёлтая кнопка окна одна («Продолжить») */
  .products .on {
    border-color: var(--accent);
    box-shadow: inset 0 -2px 0 var(--accent);
  }
  .sizes {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 6px;
  }
  .chip {
    height: auto;
    padding: 4px 10px;
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    gap: 0;
  }
  .chip.on {
    border-color: var(--accent);
    box-shadow: inset 0 -2px 0 var(--accent);
  }
  .chip small {
    font-size: 11px;
    opacity: 0.75;
  }
  .own {
    display: flex;
    align-items: center;
    gap: 6px;
  }
  .own input {
    width: 76px;
  }
  .note {
    margin: 0;
    font-size: 12px;
    color: var(--muted);
  }
  .note.bad {
    color: #e0806f;
  }
  .make:disabled {
    opacity: 0.45;
  }
</style>
