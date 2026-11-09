<script lang="ts">
  import Icon from '../ui/Icon.svelte'
  import { tipped } from '../ui/tip.svelte'
  import ColorChip from './ColorChip.svelte'

  // «Свои цвета» (109-4): всё выбранное запоминается само, «+» добавляет текущий цвет, правый щелчок и Delete убирают.
  // Список — данные программы (config.json у Go), не альбома: отмены нет.
  let {
    list,
    cur,
    onpick,
    onpreview,
    onadd,
    onforget,
  }: {
    list: readonly string[]
    cur: string
    onpick: (hex: string) => void
    onpreview: (hex: string | null) => void
    onadd: (hex: string) => void
    onforget: (hex: string) => void
  } = $props()

  const have = $derived(!!cur && list.includes(cur)) // уже в «Своих»: «+» нечего добавлять
  const off = $derived(!cur || have) // серая: aria-disabled, а не disabled — на disabled в WebView нет подсказки
  const addTip = $derived(have ? 'Уже в своих цветах' : 'Добавить текущий цвет в свои')
</script>

<div class="cap"><span>Свои цвета</span><i>запоминаются</i></div>
<div class="grid">
  <button
    type="button"
    class="add"
    class:off
    aria-label={addTip}
    aria-disabled={off}
    use:tipped={{ text: addTip }}
    onclick={() => !off && onadd(cur)}><Icon name="plus" size={14} /></button
  >
  {#each list as hex (hex)}
    <ColorChip {hex} size={22} own on={hex === cur} {onpick} {onpreview} {onforget} />
  {/each}
</div>

<style>
  .cap {
    display: flex;
    justify-content: space-between;
    margin: 8px 0 5px;
    color: var(--muted);
  }
  .cap i {
    font-style: normal;
    font-size: 11px;
  }
  .grid {
    display: flex;
    flex-wrap: wrap;
    gap: 5px;
    min-height: 22px;
  }
  .add {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 22px;
    height: 22px;
    padding: 0;
    border-radius: 5px;
    border: 1px dashed #6a6a70;
    background: transparent;
    color: var(--muted);
  }
  .add:hover:not(.off) {
    background: transparent;
    color: var(--text);
  }
  .add.off {
    opacity: 0.4;
    cursor: default;
  }
</style>
