<script lang="ts">
  import { drag } from './drag.svelte'
  import { HINT_TEXT, wheelHint } from './wheelhint.svelte'
  import { store } from '../core/store.svelte'
  import { thumbUrl } from '../core/media'
  import Icon from '../ui/Icon.svelte'
  import { Z_TOAST } from '../modal/modalstack.svelte'

  const ids = $derived(drag.payload?.kind === 'photos' ? drag.payload.ids : [])
</script>

<!-- «Призрак» под курсором при перетаскивании фото -->
{#if drag.active && drag.payload?.kind === 'photos'}
  <div class="ghost-stack" style:left="{drag.x + 14}px" style:top="{drag.y + 14}px">
    {#each ids.slice(0, 3) as id, i (id)}
      <img src={thumbUrl(id)} alt="" style:transform="translate({i * 6}px, {i * 6}px) rotate({(i - 1) * 3}deg)" />
    {/each}
    {#if drag.over || ids.length > 1}
      <!-- Плюс — SVG: символ «+» шрифта в круге съезжает с центра -->
      <span class="badge"
        >{#if drag.over}<Icon name="plus" size={12} stroke={3} />{/if}{#if ids.length > 1}<span>{ids.length}</span>{/if}</span
      >
    {/if}
    {#if wheelHint.show}<div class="hint">{HINT_TEXT}</div>{/if}
  </div>
{/if}

{#if store.toast}
  <div class="toast" style:z-index={Z_TOAST} class:error={store.toast.kind === 'error'}>
    <span>{store.toast.text}</span>
    {#if store.toast.action}
      {@const a = store.toast.action}
      <button
        class="ghost"
        onclick={() => {
          // {@const a} пересчитывается после очистки тоста и падает на null — действие берём заранее.
          const run = a.run
          store.toast = null
          run() // после очистки: действие может показать свой тост
        }}>{a.label}</button
      >
    {/if}
    <button class="x" title="Закрыть" aria-label="Закрыть" onclick={() => (store.toast = null)}><Icon name="x" size={14} /></button>
  </div>
{/if}

<style>
  .ghost-stack {
    position: fixed;
    z-index: 100;
    pointer-events: none;
    width: 90px;
    height: 90px;
  }
  .ghost-stack img {
    position: absolute;
    max-width: 80px;
    max-height: 80px;
    border: 3px solid #fff;
    box-shadow: 0 6px 20px rgba(0, 0, 0, 0.5);
  }
  .badge {
    position: absolute;
    right: -6px;
    bottom: -6px;
    min-width: 22px;
    height: 22px;
    padding: 0 5px;
    border-radius: 11px;
    background: #2bb24c;
    color: #fff;
    font-size: 12px;
    font-weight: 700;
    line-height: 1;
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 1px;
  }
  /* Подсказка колеса (095 §В): под стопкой, в стиле тоста, мышь не ловит (наследует от .ghost-stack) */
  .hint {
    position: absolute;
    left: 0;
    top: 100%;
    margin-top: 10px;
    padding: 4px 8px;
    border-radius: 6px;
    background: rgba(32, 32, 34, 0.92);
    border: 1px solid var(--line);
    color: var(--text);
    font-size: 12px;
    white-space: nowrap;
  }
  .toast {
    position: fixed;
    left: 50%;
    bottom: 24px;
    transform: translateX(-50%);
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 10px 10px 10px 16px;
    border-radius: 8px;
    background: #111;
    border: 1px solid #444;
    box-shadow: 0 10px 30px rgba(0, 0, 0, 0.5);
    font-size: 14px;
  }
  .toast.error {
    border-color: #c0392b;
  }
  .x {
    all: unset;
    cursor: pointer;
    color: #888;
    width: 26px;
    height: 26px;
    border-radius: 50%;
    display: grid;
    place-items: center;
  }
  .x:hover {
    color: #ddd;
    background: #333;
  }
</style>
