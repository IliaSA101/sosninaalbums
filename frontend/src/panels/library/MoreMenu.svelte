<script lang="ts">
  import type { Snippet } from 'svelte'
  import { modes } from '../../base/prefs/view.svelte'
  import Icon from '../../base/ui/Icon.svelte'
  import Popover from '../../base/ui/Popover.svelte'
  import { gallery } from '../gallery/gallery.svelte'

  // «⋯» в конце строки панели: группы, которым не хватило места (спека 024). Ушедшие группы рисует вызывающий.
  // hold — id справочника (033) ушедших групп: кнопка мигает вместо них.
  // open связан: полоса открывает «⋯» сама, когда окно группы, что ушла сюда, просят открыть клавишей.
  // dropBin — «⋯» принимает бросок фото вместо кнопки «Подборки», что ушла сюда (083-29); dropText — подпись цели
  // под курсором (плашка fixed: полоса обрезает всё лишнее).
  let {
    ids,
    item,
    hold = '',
    open = $bindable(false),
    dropBin = false,
    dropText = '',
  }: { ids: readonly string[]; item: Snippet<[string]>; hold?: string; open?: boolean; dropBin?: boolean; dropText?: string } = $props()

  let btn = $state<HTMLButtonElement>()

  $effect(() => {
    if (modes.big || gallery.cur || ids.length === 0) open = false
  })

  // Фокус остаётся где был: иначе Enter (хоткей «добавить выделенные») нажмёт кнопку повторно.
  const keep = (e: MouseEvent) => e.preventDefault()

  // Плашка с подписью под «⋯»: правым краем к кнопке, чтобы не уехать за окно.
  const tipAt = $derived.by(() => {
    if (!dropText || !btn) return null
    const r = btn.getBoundingClientRect()
    return { right: window.innerWidth - r.right, top: r.bottom + 6 }
  })
</script>

<button
  bind:this={btn}
  type="button"
  class="more"
  class:dropbin={dropBin}
  class:over={!!dropText}
  data-more
  data-drop={dropBin ? 'bin' : null}
  data-guide-hold={hold || null}
  title="Ещё: то, что не поместилось"
  aria-label="Ещё: то, что не поместилось"
  aria-haspopup="dialog"
  aria-expanded={open}
  onmousedown={keep}
  onclick={() => (open = !open)}><Icon name="more" size={18} /></button
>

{#if tipAt}
  <span class="droptip" role="status" style:right="{tipAt.right}px" style:top="{tipAt.top}px">{dropText}</span>
{/if}

<Popover anchor={btn} bind:open label="Ещё: то, что не поместилось" kind="more">
  <div class="box">
    {#each ids as id (id)}
      <span class="g">{@render item(id)}</span>
    {/each}
  </div>
</Popover>

<style>
  .more {
    width: 30px;
    padding: 0;
    display: grid;
    place-items: center;
    flex: none;
  }
  /* Цель броска на подборку: пунктир, как у кнопки «Подборки» (BinMenu) */
  .more.dropbin {
    outline: 1px dashed var(--accent);
    outline-offset: 2px;
  }
  .more.over {
    outline: 2px dashed var(--accent);
  }
  .droptip {
    position: fixed;
    z-index: 60;
    padding: 4px 8px;
    border-radius: 4px;
    background: var(--accent);
    color: #1e1e1e;
    font-size: 12px;
    font-weight: 600;
    white-space: nowrap;
    pointer-events: none;
  }
  .box {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 8px;
    max-width: 420px;
  }
  .g {
    display: inline-flex;
    align-items: center;
    gap: 8px;
  }
</style>
