<script lang="ts">
  import { share } from './share.svelte'
  import type { ShareLinkInfo } from './shareapi'
  import { CANCEL_LABEL, MOVED_ASK, MOVED_NEW_NOTE, MOVED_UPDATE_NOTE, NEW_ALBUM_LABEL, OPEN_LABEL, UPDATE_ALBUM_LABEL } from './sharetext'

  // Вопрос «Обновить этот альбом или выгрузить как новый?» (167-22): запись о ссылке не привязана к этому файлу — перенос,
  // копия, другой компьютер. Образец — экран «ссылка другого сервиса» (ShareDialog): ссылка текстом, пояснения, кнопки в
  // .actions, основной нет. Строки published/from/here готовит Go (без управляющих знаков); пароля и «Скопировать для клиента» нет.
  let { link, onnew }: { link: ShareLinkInfo; onnew: () => void } = $props()
  const working = $derived(share.working)
  const lines = $derived([link.published, link.from, link.here].filter(Boolean))
</script>

<div class="moved">
  {#if link.url}
    <p class="url" data-testid="share-url">{link.url}</p>
    <div class="row"><button disabled={working} onclick={() => share.openLink()}>{OPEN_LABEL}</button></div>
  {/if}
  {#each lines as line, i (i)}<p class="meta">{line}</p>{/each}
  <p class="ask">{MOVED_ASK}</p>
  <ul class="notes">
    <li>{MOVED_UPDATE_NOTE}</li>
    <li>{MOVED_NEW_NOTE}</li>
  </ul>
  <div class="actions">
    <button data-autofocus onclick={() => share.close()}>{CANCEL_LABEL}</button>
    <button disabled={working} onclick={onnew}>{NEW_ALBUM_LABEL}</button>
    <button disabled={working} onclick={() => share.start({ days: 0, choice: 'update' })}>{UPDATE_ALBUM_LABEL}</button>
  </div>
</div>

<style>
  .url {
    margin: 0 0 10px;
    padding: 6px 8px;
    background: var(--panel2);
    border-radius: 4px;
    font-family: Consolas, monospace;
    font-size: 12px;
    overflow-wrap: anywhere;
    user-select: text;
  }
  .row {
    display: flex;
    gap: 8px;
    margin: 0 0 12px;
  }
  .meta {
    margin: 0 0 6px;
    color: var(--muted);
    font-size: 12px;
    overflow-wrap: anywhere;
  }
  .ask {
    margin: 12px 0 6px;
    line-height: 1.4;
  }
  .notes {
    margin: 0 0 4px;
    padding-left: 18px;
    color: var(--muted);
    font-size: 12px;
    line-height: 1.45;
  }
  .actions {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
    margin-top: 12px;
  }
</style>
