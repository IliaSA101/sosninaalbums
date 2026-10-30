<script lang="ts">
  import Icon from '../../base/ui/Icon.svelte'
  import { isMac } from '../../base/core/platform'
  import { chordText } from '../../base/keys/chordtext'
  import type { KeymapEntry } from '../../base/types/keys'
  import { keysUi } from './keysui.svelte'
  import { reasonOf } from './keysel'

  // Сочетания команды чипами (074): щелчок по чипу — запись нового вместо него, «×» — убрать, «+» — добавить (до трёх).
  // Служебная команда — серые чипы с замком и причиной. Запись: «Нажмите сочетание… · Esc — отмена», ошибка — под чипом.
  let { e }: { e: KeymapEntry } = $props()

  const mac = isMac()
  const rec = $derived(keysUi.rec?.id === e.id ? keysUi.rec : null)
  const err = $derived(keysUi.recErr?.id === e.id ? keysUi.recErr.text : '')
  const why = $derived(reasonOf(e.defaults[0] ?? '', keysUi.ctx.locked) || 'служебная клавиша')
  const press = (fn: () => void) => (ev: MouseEvent) => {
    ev.stopPropagation()
    fn()
  }
</script>

{#snippet recording()}
  <span class="chip rec" data-chip>{rec?.partial || 'Нажмите сочетание…'} · Esc — отмена</span>
{/snippet}

{#if e.keys.length === 0 && !rec}<span class="none">нет клавиши</span>{/if}
{#each e.keys as k (k)}
  {#if rec && rec.old === k}
    {@render recording()}
  {:else if e.fixed}
    <span class="chip fixed" title="Служебная: {why}">{chordText(k, mac)}</span>
  {:else}
    <span class="chip" class:cust={e.custom} data-chip>
      <button title="Изменить сочетание" onclick={press(() => keysUi.startRec(e.id, k))}>{chordText(k, mac)}</button>
      <button class="cx" title="Убрать сочетание" aria-label="Убрать" onclick={press(() => void keysUi.remove(e.id, k))}>×</button>
    </span>
  {/if}
{/each}
{#if !e.fixed}
  {#if rec && rec.old === ''}
    {@render recording()}
  {:else if e.keys.length < 3}
    <button
      class="chip plus"
      data-chip
      title="Добавить сочетание"
      aria-label="Добавить сочетание"
      onclick={press(() => keysUi.startRec(e.id, ''))}
    >
      +
    </button>
  {/if}
{:else}
  <span class="lk" title="Служебная: {why}"><Icon name="lock" size={12} /></span>
{/if}
{#if err}<span class="err" role="alert">{err}</span>{/if}

<style>
  .chip {
    display: inline-flex;
    align-items: center;
    height: 22px;
    border: 1px solid var(--line);
    background: #3a3a3e;
    border-radius: 4px;
    font-weight: 600;
    font-size: 12px;
    color: var(--text);
    white-space: nowrap;
    overflow: hidden;
  }
  .chip button {
    height: 100%;
    padding: 0 6px;
    border-radius: 0;
    background: transparent;
    font-weight: 600;
    font-size: 12px;
    border: 0;
  }
  .chip button:hover:not(:disabled) {
    background: #52525a;
  }
  .chip .cx {
    padding: 0 5px;
    color: var(--muted);
    font-weight: 400;
    display: none;
    border-left: 1px solid var(--line);
  }
  .chip:hover .cx,
  .chip:focus-within .cx {
    display: inline-block;
  }
  .chip.fixed {
    padding: 0 6px;
    color: var(--muted);
    background: #2a2a2d;
  }
  .chip.plus {
    padding: 0 8px;
    color: var(--muted);
    background: transparent;
    border: 1px dashed var(--line);
  }
  .chip.plus:hover:not(:disabled) {
    color: var(--text);
    background: #38383c;
  }
  .chip.rec {
    padding: 0 8px;
    border-color: var(--accent);
    color: var(--accent);
    background: transparent;
    animation: pulse 1.1s ease-in-out infinite;
  }
  .chip.cust {
    border-color: #6f6a2a;
  }
  @keyframes pulse {
    50% {
      background: rgba(245, 217, 10, 0.12);
    }
  }
  .none {
    color: var(--muted);
    font-size: 12px;
  }
  .lk {
    display: inline-flex;
    color: var(--muted);
  }
  .err {
    flex-basis: 100%;
    text-align: right;
    color: var(--lbl-red);
    font-size: 12px;
  }
</style>
