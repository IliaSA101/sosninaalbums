<script lang="ts">
  import Icon from '../../base/ui/Icon.svelte'
  import { keymap } from '../../base/keys/keymap.svelte'
  import type { Bin } from '../../base/core/types'
  import BinName from './BinName.svelte'
  import { countText, countTip, minusText, plusText, type BinCount } from './bins'

  // Строка окна «Подборки» (083): показ щелчком по имени, «± N» по выделению, ✎ и двойной щелчок — имя на месте, ×.
  let {
    bin,
    n,
    current,
    target,
    sel,
    action,
    editing,
    err,
    onshow,
    onset,
    onedit,
    onrename,
    oncancel,
    onremove,
  }: {
    bin: Bin
    n: BinCount
    current: boolean // лента показывает эту подборку
    target: boolean // целевая для B и переноса
    sel: number // сколько выделено в ленте
    action: 'add' | 'remove'
    editing: boolean
    err: string
    onshow: () => void
    onset: () => void
    onedit: () => void
    onrename: (name: string) => void
    oncancel: () => void
    onremove: () => void
  } = $props()

  // Кнопки внутри окна не гасят фокус и не начинают перенос ленты.
  const stop = (e: PointerEvent) => e.stopPropagation()
  const setTip = $derived(action === 'add' ? `Добавить выделенные (${sel}) в «${bin.name}»` : `Убрать выделенные (${sel}) из «${bin.name}»`)
</script>

<li class:cur={current}>
  <span class="dot" aria-hidden="true">{current ? '●' : ''}</span>
  {#if editing}
    <BinName value={bin.name} label="Имя подборки" {err} onsubmit={onrename} {oncancel} />
  {:else}
    <button type="button" class="name" title={bin.name} aria-current={current} onpointerdown={stop} onclick={onshow} ondblclick={onedit}
      >{bin.name}</button
    >
    <span class="n" title={countTip(n)}>{countText(n)}</span>
    {#if sel > 0}
      <button type="button" class="set" title={target ? keymap.tip(setTip, 'binToggle') : setTip} onpointerdown={stop} onclick={onset}
        >{action === 'add' ? plusText(sel) : minusText(sel)}</button
      >
    {/if}
    <button
      type="button"
      class="ghost ic"
      title="Переименовать"
      aria-label="Переименовать подборку «{bin.name}»"
      onpointerdown={stop}
      onclick={onedit}>✎</button
    >
    <button
      type="button"
      class="ghost ic"
      title="Удалить подборку (фото останутся)"
      aria-label="Удалить подборку «{bin.name}»"
      onpointerdown={stop}
      onclick={onremove}><Icon name="x" size={12} stroke={2.4} /></button
    >
  {/if}
</li>

<style>
  li {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 6px;
    padding: 2px 0 2px 4px;
  }
  li.cur {
    background: rgba(255, 255, 255, 0.05);
  }
  .dot {
    width: 10px;
    color: var(--accent);
    font-size: 10px;
  }
  .name {
    flex: 1;
    min-width: 0;
    text-align: left;
    background: transparent;
    padding: 0 4px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .n {
    color: var(--muted);
    font-variant-numeric: tabular-nums;
  }
  .set {
    background: #4a4a2a;
    color: var(--accent);
    padding: 0 6px;
  }
  .ic {
    width: 22px;
    height: 22px;
    padding: 0;
    display: grid;
    place-items: center;
    color: var(--muted);
  }
  li button {
    height: 22px;
    font-size: 12px;
  }
</style>
