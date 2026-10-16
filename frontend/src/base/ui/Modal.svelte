<script lang="ts">
  import { untrack, type Snippet } from 'svelte'
  import Icon from './Icon.svelte'
  import { modalStack } from './modalstack.svelte'

  // Окно поверх интерфейса (не нативное — работает и в браузере dev). Esc и клик по фону закрывают;
  // Esc не доходит до хоткеев App (не закрывает галерею и не снимает выделение). width — своя ширина, если 380/460 мало.
  // size: 'large' — окно с предпросмотром (до 1280 px по ширине экрана, 031 «Настроить виньетку»: сетка шаблонов в 4–5 колонок).
  let {
    title,
    onclose,
    wide = false,
    size,
    width,
    children,
  }: {
    title: string
    onclose: () => void
    wide?: boolean
    size?: 'large'
    width?: number
    children: Snippet
  } = $props()

  let box = $state<HTMLElement>()
  // Порядок окон — modalstack (справочник поверх стартового: Esc и фокус только верхнему).
  let me = $state(0)
  const top = $derived(me > 0 && modalStack.isTop(me))
  $effect(() => {
    const id = untrack(() => modalStack.push())
    me = id
    return () => modalStack.remove(id)
  })
  // Фокус в окне: Esc и Enter ловит оно, а не страница под ним. Поле с data-autofocus получает его первым.
  // Окно под закрывшимся верхним забирает фокус обратно.
  $effect(() => {
    if (box && top) (box.querySelector<HTMLElement>('[data-autofocus]') ?? box).focus()
  })
</script>

<!-- Захват на window: кнопка, на которой стоял фокус, могла исчезнуть (смена содержимого окна), и клавиши ушли бы в страницу под ним -->
<svelte:window
  onkeydowncapture={(e) => {
    if (!top) return
    if (e.key === 'Escape') {
      if (e.defaultPrevented) return // Esc уже закрыл вложенное окно (Popover внутри окна)
      e.stopPropagation()
      e.preventDefault()
      onclose()
    } else if (box && !box.contains(document.activeElement)) {
      e.stopPropagation()
      box.focus()
    }
  }}
/>

<div
  class="back"
  style:z-index={80 + Math.max(modalStack.depth(me), 0)}
  role="presentation"
  onpointerdown={(e) => e.target === e.currentTarget && onclose()}
>
  <div
    class="box"
    class:wide
    class:large={size === 'large'}
    style:width={width ? `${width}px` : undefined}
    role="dialog"
    aria-modal="true"
    aria-label={title}
    tabindex="-1"
    bind:this={box}
  >
    <header>
      <span>{title}</span>
      <button class="x" onclick={onclose} title="Закрыть (Esc)" aria-label="Закрыть"><Icon name="x" size={14} /></button>
    </header>
    {@render children()}
  </div>
</div>

<style>
  .back {
    position: fixed;
    inset: 0;
    z-index: 80;
    display: grid;
    place-items: center;
    background: rgba(0, 0, 0, 0.5);
  }
  .box {
    width: 380px;
    max-width: calc(100vw - 32px);
    max-height: calc(100vh - 32px);
    overflow: auto;
    padding: 12px 16px 14px;
    background: var(--panel);
    border: 1px solid var(--line);
    border-radius: 10px;
    box-shadow: 0 16px 48px rgba(0, 0, 0, 0.55);
    outline: none;
    font-size: 13px;
  }
  .box.wide {
    width: 460px;
  }
  .box.large {
    width: min(1280px, calc(100vw - 64px));
  }
  header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 10px;
    font-size: 15px;
    font-weight: 600;
  }
  .x {
    width: 26px;
    height: 26px;
    padding: 0;
    display: grid;
    place-items: center;
    background: transparent;
  }
</style>
