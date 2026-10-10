<script lang="ts">
  import type { LayoutScope } from '../../base/core/types'
  import { tip, tipped } from '../../base/ui/tip.svelte'
  import Icon, { type IconName } from '../../base/ui/Icon.svelte'

  // Меню «Отразить»: страница или весь разворот. side: 'down' — кнопка текстом в LayoutBar, список
  // под ней; 'right' — значок в ленте правки (EditRail даёт ему стиль через класс rb), список справа;
  // label — подпись справа от значка (лента в один столбец).
  let { onpick, side = 'down', label = '' }: { onpick: (scope: LayoutScope) => void; side?: 'down' | 'right'; label?: string } = $props()

  let open = $state(false)
  let at = $state({ x: 0, y: 0 })
  let btn = $state<HTMLElement>()
  let menu = $state<HTMLElement>()
  const items: { scope: LayoutScope; label: string; icon: IconName }[] = [
    { scope: 'left', label: 'Левая страница', icon: 'mirror-left' },
    { scope: 'right', label: 'Правая страница', icon: 'mirror-right' },
    { scope: 'spread', label: 'Разворот', icon: 'mirror-spread' },
  ]
  const stop = (e: PointerEvent) => e.stopPropagation()

  // Список переезжает в body: полоса над разворотом обрезается (overflow) и сдвинута transform,
  // из-за чего position: fixed внутри неё считался бы от группы, а не от окна.
  function portal(node: HTMLElement) {
    document.body.appendChild(node)
    return { destroy: () => node.remove() }
  }

  function toggle() {
    if (!open) {
      const r = btn!.getBoundingClientRect()
      at = side === 'right' ? { x: r.right + 4, y: r.top } : { x: r.left, y: r.bottom + 2 }
      tip.hide()
    }
    open = !open
  }

  function pick(scope: LayoutScope) {
    open = false
    onpick(scope)
  }
</script>

<svelte:window
  onpointerdown={(e) => {
    const t = e.target as Node
    if (open && !btn?.contains(t) && !menu?.contains(t)) open = false
  }}
  onkeydown={(e) => {
    if (open && e.key === 'Escape') {
      open = false
      e.stopPropagation()
    }
  }}
  onresize={() => (open = false)}
/>

{#if side === 'right'}
  <button
    bind:this={btn}
    class="rb"
    onpointerdown={stop}
    onclick={toggle}
    use:tipped={{ text: open ? '' : 'Отразить раскладку ▸', side: 'right' }}
    aria-haspopup="menu"
    aria-expanded={open}
    aria-label="Отразить раскладку"
    ><Icon name="mirror-layout" size={18} />{#if label}<span class="rl">{label}</span>{/if}</button
  >
{:else}
  <button
    bind:this={btn}
    class="text"
    onpointerdown={stop}
    onclick={toggle}
    aria-haspopup="menu"
    aria-expanded={open}
    title="Отразить раскладку слева направо">Отразить ▾</button
  >
{/if}
{#if open}
  <div class="menu" role="menu" bind:this={menu} use:portal style:left="{at.x}px" style:top="{at.y}px">
    {#each items as it (it.scope)}
      <button role="menuitem" onclick={() => pick(it.scope)}><Icon name={it.icon} size={16} />{it.label}</button>
    {/each}
  </div>
{/if}

<style>
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
  .text {
    height: 26px;
    padding: 0 7px;
    font-size: 12px;
    white-space: nowrap;
    background: transparent;
    color: #cfcfd4;
    border-radius: 4px;
  }
</style>
