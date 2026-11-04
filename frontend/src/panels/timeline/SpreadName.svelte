<script lang="ts">
  import { store } from '../../base/core/store.svelte'
  import { NAME_MAX, nameChanged, normName } from './spreadname'

  // Поле названия разворота на месте номера в ленте (спека 008).
  let { id, name, onclose }: { id: string; name: string; onclose: () => void } = $props()

  // Начальное значение — название на момент открытия; дальше поле владеет текстом само.
  // svelte-ignore state_referenced_locally
  let value = $state(name)
  let done = false // Enter/Esc уже сработали: blur от удаления поля не должен сохранять второй раз

  function focus(el: HTMLInputElement) {
    el.focus()
    el.select()
  }

  function commit() {
    if (done) return
    done = true
    if (nameChanged(name, value)) store.renameSpread(id, normName(value))
    onclose()
  }

  function cancel() {
    done = true
    onclose()
  }

  function key(e: KeyboardEvent) {
    // Хоткеи окна в поле не нужны; Esc не должен ещё и снять выделение.
    e.stopPropagation()
    if (e.key === 'Enter') {
      e.preventDefault()
      commit()
    } else if (e.key === 'Escape') {
      e.preventDefault()
      cancel()
    }
  }

  // Лента перетаскивает группу за pointerdown, а клик ведёт на разворот — поле их не пускает.
  const stop = (e: Event) => e.stopPropagation()
</script>

<input
  class="name"
  type="text"
  maxlength={NAME_MAX}
  placeholder="Название"
  aria-label="Название разворота"
  bind:value
  use:focus
  onpointerdown={stop}
  onclick={stop}
  ondblclick={stop}
  onkeydown={key}
  onblur={commit}
/>

<style>
  .name {
    display: block;
    box-sizing: border-box;
    width: 100%; /* ширину группе на время правки задаёт .renaming в Timeline */
    height: 18px;
    margin-top: 1px;
    padding: 0 4px;
    font-size: 11px;
    line-height: 16px;
    text-align: center;
  }
</style>
