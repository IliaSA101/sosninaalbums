<script lang="ts">
  import { MAX_BIN_NAME } from './bins'

  // Поле имени подборки на месте строки (новая и переименование, 083): текст выделен, Enter — готово, Esc и уход
  // фокуса — отмена. data-barpop: пока поле есть, Popover не закрывается на Esc — Esc отменяет только ввод.
  let {
    value,
    label,
    err = '',
    onsubmit,
    oncancel,
  }: { value: string; label: string; err?: string; onsubmit: (name: string) => void; oncancel: () => void } = $props()

  let el = $state<HTMLInputElement>()
  let text = $state('')
  let first = true
  $effect(() => {
    if (!el || !first) return
    first = false
    text = value
    el.focus()
    el.select()
  })

  function key(e: KeyboardEvent) {
    if (e.key === 'Enter') {
      e.preventDefault()
      e.stopPropagation()
      onsubmit(text)
    } else if (e.key === 'Escape') {
      e.preventDefault()
      e.stopPropagation()
      oncancel()
    }
  }
</script>

<input
  bind:this={el}
  bind:value={text}
  class="rename"
  class:bad={err !== ''}
  data-barpop="bin-name"
  type="text"
  maxlength={MAX_BIN_NAME}
  aria-label={label}
  aria-invalid={err !== ''}
  onkeydown={key}
  onblur={oncancel}
  onpointerdown={(e) => e.stopPropagation()}
/>
{#if err}<div class="err" role="alert">{err}</div>{/if}

<style>
  .rename {
    flex: 1;
    min-width: 0;
    height: 22px;
    padding: 0 6px;
    font: inherit;
    font-size: 12px;
    color: var(--text);
    background: #3a3a3e;
    border: 1px solid var(--accent);
    border-radius: 4px;
  }
  .rename.bad {
    border-color: #ff8a8a;
  }
  .err {
    flex-basis: 100%;
    font-size: 12px;
    color: #ff8a8a;
  }
</style>
