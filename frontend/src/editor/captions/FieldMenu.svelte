<script lang="ts">
  import { token, type Field } from './fields'

  // Меню «Вставить поле ▾» (030): поля листа (`fields`, от Go), `{Фамилия}`… Кнопка и пункты не забирают фокус
  // (mousedown гасится), поэтому курсор остаётся в редакторе или в поле шаблона. Список переезжает
  // в body: карточка прокручивается и обрезает fixed-потомков с transform; data-text-ui — щелчок
  // по списку не снимает выделение текста.
  let { onpick, fields, label = 'Вставить поле' }: { onpick: (f: Field) => void; fields: readonly Field[]; label?: string } = $props()

  let open = $state(false)
  let at = $state({ x: 0, y: 0 })
  let btn = $state<HTMLElement>()
  let menu = $state<HTMLElement>()
  const keepFocus = (e: MouseEvent) => e.preventDefault()

  function portal(node: HTMLElement) {
    document.body.appendChild(node)
    return { destroy: () => node.remove() }
  }

  function toggle() {
    if (!open) {
      const r = btn!.getBoundingClientRect()
      at = { x: r.left, y: r.bottom + 2 }
    }
    open = !open
  }

  function pick(f: Field) {
    open = false
    onpick(f)
  }
</script>

<svelte:window
  onpointerdown={(e) => {
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
  class="fld"
  data-guide="field-menu"
  data-text-ui
  aria-haspopup="menu"
  aria-expanded={open}
  title="В текст попадёт поле: имя с фото или поле шапки виньетки"
  onmousedown={keepFocus}
  onclick={toggle}>{label} ▾</button
>
{#if open}
  <div class="menu" role="menu" bind:this={menu} use:portal data-text-ui style:left="{at.x}px" style:top="{at.y}px">
    {#each fields as f (f)}
      <button role="menuitem" onmousedown={keepFocus} onclick={() => pick(f)}>{token(f)}</button>
    {/each}
  </div>
{/if}

<style>
  .fld {
    margin-top: 8px;
    width: 100%;
    height: 24px;
    padding: 0 8px;
    font-size: 12px;
    background: #3a3a3e;
  }
  .menu {
    position: fixed;
    z-index: 90;
    min-width: 140px;
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
    height: 26px;
    padding: 0 10px;
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
