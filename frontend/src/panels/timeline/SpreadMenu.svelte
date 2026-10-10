<script lang="ts">
  import Icon from '../../base/ui/Icon.svelte'
  import { deleteLabel, menuPos } from './spreadmenu'

  // Меню правого щелчка по группе ленты (пока один пункт). Esc и щелчок мимо закрывают;
  // окно держится внутри экрана. Устройство — как у PhotoMenu. count > 1 — выделено несколько групп.
  let { x, y, count = 1, onDelete, onclose }: { x: number; y: number; count?: number; onDelete: () => void; onclose: () => void } = $props()

  let box = $state<HTMLElement>()
  let w = $state(0)
  let h = $state(0)
  const pos = $derived(menuPos(x, y, { w, h }, { w: window.innerWidth, h: window.innerHeight }))

  // Фокус на пункт: Enter работает сразу.
  $effect(() => {
    box?.querySelector<HTMLElement>('button')?.focus()
  })

  function key(e: KeyboardEvent) {
    // Пока меню открыто, горячие клавиши приложения не работают (как в PhotoMenu).
    e.stopPropagation()
    if (e.key === 'Escape') {
      e.preventDefault()
      onclose()
    }
  }

  function outside(e: PointerEvent) {
    if (box && !box.contains(e.target as Node)) onclose()
  }
</script>

<svelte:window onkeydowncapture={key} onpointerdowncapture={outside} onresize={onclose} onblur={onclose} />

<div
  bind:this={box}
  bind:clientWidth={w}
  bind:clientHeight={h}
  class="menu"
  style:left="{pos.x}px"
  style:top="{pos.y}px"
  style:visibility={h ? undefined : 'hidden'}
  role="menu"
  tabindex="-1"
  data-chrome
  data-text-ui
  oncontextmenu={(e) => e.preventDefault()}
>
  <button class="danger" role="menuitem" onclick={onDelete}>
    <Icon name="spread-delete" size={16} stroke={1.8} /><span>{deleteLabel(count)}</span>
  </button>
</div>

<style>
  .menu {
    position: fixed;
    z-index: 70;
    min-width: 200px;
    padding: 4px;
    box-sizing: border-box;
    background: rgba(30, 30, 32, 0.97);
    border: 1px solid var(--line);
    border-radius: 6px;
    box-shadow: 0 6px 20px rgba(0, 0, 0, 0.5);
    font-size: 13px;
    user-select: none;
  }
  button {
    width: 100%;
    height: 28px;
    padding: 0 10px;
    display: flex;
    align-items: center;
    gap: 8px;
    text-align: left;
    white-space: nowrap;
    background: transparent;
    color: var(--text);
    border-radius: 4px;
  }
  button:hover,
  button:focus-visible {
    background: #38383c;
    color: #fff;
  }
  button.danger:hover,
  button.danger:focus-visible {
    color: #ff8a8a;
  }
</style>
