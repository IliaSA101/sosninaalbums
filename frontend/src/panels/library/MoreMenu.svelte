<script lang="ts">
  import type { Snippet } from 'svelte'
  import { modes } from '../../base/prefs/view.svelte'
  import Icon from '../../base/ui/Icon.svelte'
  import Popover from '../../base/ui/Popover.svelte'
  import { gallery } from '../layouts/gallery.svelte'

  // «⋯» в конце строки панели: группы, которым не хватило места (спека 024). Ушедшие группы рисует вызывающий.
  // hold — id справочника (033) ушедших групп: кнопка мигает вместо них.
  let { ids, item, hold = '' }: { ids: readonly string[]; item: Snippet<[string]>; hold?: string } = $props()

  let open = $state(false)
  let btn = $state<HTMLButtonElement>()

  $effect(() => {
    if (modes.big || gallery.cur || ids.length === 0) open = false
  })

  // Фокус остаётся где был: иначе Enter (хоткей «добавить выделенные») нажмёт кнопку повторно.
  const keep = (e: MouseEvent) => e.preventDefault()
</script>

<button
  bind:this={btn}
  type="button"
  class="more"
  data-more
  data-guide-hold={hold || null}
  title="Ещё: то, что не поместилось"
  aria-label="Ещё: то, что не поместилось"
  aria-haspopup="dialog"
  aria-expanded={open}
  onmousedown={keep}
  onclick={() => (open = !open)}><Icon name="more" size={18} /></button
>

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
