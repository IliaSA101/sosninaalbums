<script lang="ts">
  import { untrack } from 'svelte'
  import { share } from './share.svelte'
  import { CANCEL_LABEL, FIND_LINE, RETRY_LABEL } from './sharetext'

  // Экран поиска (167): файл помнит выгруженный альбом — ищем его среди альбомов подключённого сервиса. Вызов один на показ;
  // итог (найден → moved, не найден, сбой) придёт в виде и сменит экран. Отказ самого вызова — строка и «Повторить».
  $effect(() => {
    untrack(() => void share.find())
  })
</script>

<div class="find">
  <progress aria-label={FIND_LINE}></progress>
  <p class="line" role="status">{FIND_LINE}</p>
  {#if share.findError}<p class="err" role="alert">{share.findError}</p>{/if}
  <div class="actions">
    <button data-autofocus onclick={() => share.close()}>{CANCEL_LABEL}</button>
    {#if share.findError}<button disabled={share.finding} onclick={() => share.find()}>{RETRY_LABEL}</button>{/if}
  </div>
</div>

<style>
  progress {
    width: 100%;
    height: 8px;
    accent-color: var(--accent);
  }
  .line {
    margin: 10px 0 4px;
    font-size: 13px;
  }
  .err {
    margin: 0 0 6px;
    color: #e5645a;
    font-size: 12px;
  }
  .actions {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
    margin-top: 12px;
  }
</style>
