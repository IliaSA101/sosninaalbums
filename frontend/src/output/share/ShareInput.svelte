<script lang="ts">
  // Текстовое поле окна «Отправить по ссылке»: одно место для вида и для autocomplete="off" (WebView2 по умолчанию
  // запоминает поля, а здесь токен и пароль клиента — разбор 146 №32). secret — токен: точки вместо знаков.
  let {
    value = $bindable(''),
    label,
    placeholder = '',
    secret = false,
    invalid = false,
    onenter,
  }: {
    value?: string
    label: string // aria-label: подпись рядом — в ExportGroup, не <label>
    placeholder?: string
    secret?: boolean
    invalid?: boolean
    onenter?: () => void
  } = $props()

  function key(e: KeyboardEvent) {
    if (e.key !== 'Enter' || !onenter) return
    e.preventDefault()
    onenter()
  }
</script>

<input
  class="inp"
  class:bad={invalid}
  type={secret ? 'password' : 'text'}
  aria-label={label}
  aria-invalid={invalid}
  {placeholder}
  autocomplete="off"
  autocapitalize="off"
  spellcheck="false"
  bind:value
  onkeydown={key}
/>

<style>
  .inp {
    flex: 1 1 220px;
    min-width: 0;
    height: 26px;
    padding: 0 8px;
    font: inherit;
    background: #3a3a3e;
    color: var(--text);
    border: 1px solid var(--line);
    border-radius: 6px;
    user-select: text;
  }
  .inp.bad {
    border-color: #c0392b;
  }
</style>
