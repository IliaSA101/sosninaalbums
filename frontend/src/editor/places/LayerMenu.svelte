<script lang="ts">
  import { LAYER_OPS, type LayerOp } from './minipanel'
  import { tip, tipped } from '../../base/ui/tip.svelte'
  import Icon from '../../base/ui/Icon.svelte'

  // Значок «Слой ▾» мини-панели и список из четырёх действий. Список уходит в body (portal): полоса
  // стоит внутри разворота с transform, и position: fixed считался бы от неё, а не от окна.
  let { onpick, label = 'Слой' }: { onpick: (op: LayerOp) => void; label?: string } = $props()

  let open = $state(false)
  let at = $state({ x: 0, y: 0 })
  let btn = $state<HTMLElement>()
  let menu = $state<HTMLElement>()

  function portal(node: HTMLElement) {
    document.body.appendChild(node)
    return { destroy: () => node.remove() }
  }

  function toggle() {
    if (!open) {
      const r = btn!.getBoundingClientRect()
      at = { x: r.left, y: r.bottom + 4 }
      tip.hide()
    }
    open = !open
  }

  function pick(op: LayerOp) {
    open = false
    onpick(op)
  }
</script>

<!-- Захват: полоса гасит pointerdown на себе, а щелчок по ней мимо значка тоже закрывает список -->
<svelte:window
  onpointerdowncapture={(e) => {
    const t = e.target as Node
    if (open && !btn?.contains(t) && !menu?.contains(t)) open = false
  }}
  onkeydowncapture={(e) => {
    if (open && e.key === 'Escape') {
      open = false
      e.stopPropagation()
      e.preventDefault() // Esc занят: режим шкалы и прочие его не берут
    }
  }}
  onresize={() => (open = false)}
/>

<button
  bind:this={btn}
  class="mb"
  class:on={open}
  onclick={toggle}
  use:tipped={{ text: open ? '' : label }}
  aria-haspopup="menu"
  aria-expanded={open}
  aria-label={label}><Icon name="layers" size={14} /><span class="caret">▾</span></button
>
{#if open}
  <div class="menu" role="menu" bind:this={menu} use:portal style:left="{at.x}px" style:top="{at.y}px">
    {#each LAYER_OPS as it (it.op)}
      <button role="menuitem" onclick={() => pick(it.op)}><Icon name={it.icon} size={16} />{it.label}</button>
    {/each}
  </div>
{/if}

<style>
  .mb {
    height: 22px;
    padding: 0 6px;
    display: flex;
    align-items: center;
    gap: 2px;
    background: transparent;
    color: #cfcfd4;
    border-radius: 4px;
  }
  .mb:hover,
  .mb.on {
    background: #38383c;
    color: #fff;
  }
  .caret {
    font-size: 9px;
  }
  .menu {
    position: fixed;
    z-index: 90;
    min-width: 150px;
    padding: 4px;
    display: flex;
    flex-direction: column;
    gap: 2px;
    background: #2a2a2d;
    border: 1px solid var(--line);
    border-radius: 6px;
    box-shadow: 0 6px 18px rgba(0, 0, 0, 0.45);
  }
  .menu button {
    height: 28px;
    padding: 0 10px;
    display: flex;
    align-items: center;
    gap: 8px;
    text-align: left;
    font-size: 12px;
    background: transparent;
    color: #cfcfd4;
    border-radius: 4px;
  }
  .menu button:hover {
    background: #3a3a3e;
  }
</style>
