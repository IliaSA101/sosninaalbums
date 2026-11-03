<script lang="ts">
  import ExportGroup from '../export/ExportGroup.svelte'
  import ShareInput from './ShareInput.svelte'
  import { share } from './share.svelte'
  import {
    CANCEL_LABEL,
    CONNECTING_LABEL,
    CONNECT_HINT,
    CONNECT_LABEL,
    TOKEN_LABEL,
    URL_LABEL,
    URL_PLACEHOLDER,
    dataHint,
  } from './sharetext'

  // quiet: над подключением уже есть ссылка с «Скопировать для клиента» — главная кнопка экрана там, «Подключить» обычная.
  let { quiet = false }: { quiet?: boolean } = $props()
  // Подключение к сервису (один раз): адрес и токен. Проверку делает Go; его отказ — под полями (ошибки словами Go).
  // До подключения вместо имени сервера в строке про данные — «этот сервер» (152-ФЗ, 0.32-146-28).
  let url = $state(share.view?.service.url ?? '')
  let token = $state('')
  const working = $derived(share.working)
  const can = $derived(!working && url.trim() !== '' && token !== '')

  function go() {
    if (can) void share.connect(url, token)
  }
</script>

<ExportGroup cap={URL_LABEL}>
  <ShareInput bind:value={url} label={URL_LABEL} placeholder={URL_PLACEHOLDER} onenter={go} invalid={!!share.error} />
</ExportGroup>
<ExportGroup cap={TOKEN_LABEL}>
  <ShareInput bind:value={token} label={TOKEN_LABEL} secret onenter={go} invalid={!!share.error} />
</ExportGroup>
{#if share.error}<p class="err" role="alert">{share.error}</p>{/if}
<p class="hint">{CONNECT_HINT}</p>
<p class="hint">{dataHint('')}</p>

<div class="actions">
  <button class="ghost" onclick={() => share.close()}>{CANCEL_LABEL}</button>
  <button class={quiet ? undefined : 'primary'} data-autofocus={quiet ? undefined : ''} disabled={!can} onclick={go}
    >{working ? CONNECTING_LABEL : CONNECT_LABEL}</button
  >
</div>

<style>
  .hint {
    color: var(--muted);
    font-size: 12px;
    line-height: 1.45;
    margin: 0 0 8px;
  }
  .err {
    margin: -6px 0 8px;
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
