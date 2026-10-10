<script lang="ts">
  import type { Snippet } from 'svelte'
  import Icon from './Icon.svelte'

  // Окно поверх интерфейса (не нативное — работает и в браузере dev). Esc и клик по фону закрывают;
  // Esc не доходит до хоткеев App (не закрывает галерею и не снимает выделение).
  let { title, onclose, wide = false, children }: { title: string; onclose: () => void; wide?: boolean; children: Snippet } = $props()

  let box = $state<HTMLElement>()
  // Фокус в окне: Esc и Enter ловит оно, а не страница под ним. Поле с data-autofocus получает его первым.
  $effect(() => {
    if (box) (box.querySelector<HTMLElement>('[data-autofocus]') ?? box).focus()
  })
</script>

<!-- Захват на window: кнопка, на которой стоял фокус, могла исчезнуть (смена содержимого окна), и клавиши ушли бы в страницу под ним -->
<svelte:window
  onkeydowncapture={(e) => {
    if (e.key === 'Escape') {
      e.stopPropagation()
      e.preventDefault()
      onclose()
    } else if (box && !box.contains(document.activeElement)) {
      e.stopPropagation()
      box.focus()
    }
  }}
/>

<div class="back" role="presentation" onpointerdown={(e) => e.target === e.currentTarget && onclose()}>
  <div class="box" class:wide role="dialog" aria-modal="true" aria-label={title} tabindex="-1" bind:this={box}>
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
