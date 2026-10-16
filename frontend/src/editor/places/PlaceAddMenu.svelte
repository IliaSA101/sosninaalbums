<script lang="ts">
  import { tip, tipped } from '../../base/ui/tip.svelte'
  import Icon, { type IconName } from '../../base/ui/Icon.svelte'

  // Меню «Место ▾» в ленте правки (044): «Прямоугольное» / «Круглое». Устроено как «Отразить» (MirrorMenu,
  // side='right'): кнопка получает стиль `rb` от EditRail, список справа через portal, Esc и щелчок вне закрывают.
  // label — подпись справа от значка (лента в один столбец).
  let { onpick, title, label = '' }: { onpick: (shape: '' | 'oval') => void; title: string; label?: string } = $props()

  let open = $state(false)
  let at = $state({ x: 0, y: 0 })
  let btn = $state<HTMLElement>()
  let menu = $state<HTMLElement>()
  const items: { shape: '' | 'oval'; label: string; icon: IconName }[] = [
    { shape: '', label: 'Прямоугольное', icon: 'place-add' },
    { shape: 'oval', label: 'Круглое', icon: 'place-round' },
  ]
  const stop = (e: PointerEvent) => e.stopPropagation()
  // Кнопки ленты не забирают фокус у редактора текста: курсор остаётся в нём.
  const keepFocus = (e: MouseEvent) => e.preventDefault()

  // Список переезжает в body: лента обрезается и сдвинута transform (как у MirrorMenu).
  function portal(node: HTMLElement) {
    document.body.appendChild(node)
    return { destroy: () => node.remove() }
  }

  function toggle() {
    if (!open) {
      const r = btn!.getBoundingClientRect()
      at = { x: r.right + 4, y: r.top }
      tip.hide()
    }
    open = !open
  }

  function pick(shape: '' | 'oval') {
    open = false
    onpick(shape)
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

<button
  bind:this={btn}
  class="rb"
  onmousedown={keepFocus}
  onpointerdown={stop}
  onclick={toggle}
  use:tipped={{ text: open ? '' : `${title} ▸`, side: 'right' }}
  aria-haspopup="menu"
  aria-expanded={open}
  aria-label={title}
  ><Icon name="place-add" size={18} />{#if label}<span class="rl">{label}</span>{/if}</button
>
{#if open}
  <div class="menu" role="menu" bind:this={menu} use:portal style:left="{at.x}px" style:top="{at.y}px">
    {#each items as it (it.shape)}
      <button role="menuitem" onmousedown={keepFocus} onclick={() => pick(it.shape)}><Icon name={it.icon} size={16} />{it.label}</button>
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
</style>
