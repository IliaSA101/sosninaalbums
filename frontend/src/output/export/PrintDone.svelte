<script lang="ts">
  import { Api, store } from '../../base/core/store.svelte'
  import { jobs } from '../../base/core/jobs.svelte'
  import { errText } from '../../base/words/errtext'
  import Modal from '../../base/modal/Modal.svelte'

  // Окно «Готово для …» (спека 047, решение 57; 081): памятка нужна, пока фотограф заполняет заказ на сайте,
  // а тост гаснет через 8 с. Открыто, пока jobs.exportMemo не null; строки памятки приходят из Go (export:done),
  // заголовок, путь на сайте и кнопка — из плана, запомненного при запуске выгрузки (тексты готовит Go).
  const memo = $derived(jobs.exportMemo ?? [])
  const plan = $derived(jobs.exportPrint)

  function close() {
    jobs.exportMemo = null
    jobs.exportNote = ''
    jobs.exportPrint = null
  }
  // Окно остаётся открытым: фотограф идёт на сайт и возвращается к памятке.
  async function open(call: () => Promise<void>) {
    try {
      await call()
    } catch (e) {
      store.notify(errText(e), 'error')
    }
  }
  // Кнопка сайта — по типографии плана: адрес Go берёт из её профиля (неизвестный id и Нетпринт — ничего не открывает).
  const openSite = () => open(() => Api.OpenPrintSite(plan?.kind ?? ''))
</script>

{#if jobs.exportMemo}
  <Modal title={plan?.done || 'Готово'} onclose={close} wide>
    <ul class="memo">
      {#each memo as line (line)}<li>{line}</li>{/each}
    </ul>
    {#if plan?.site}<p class="site">{plan.site}</p>{/if}
    {#if jobs.exportNote}<p class="hint">{jobs.exportNote}.</p>{/if}

    <div class="actions">
      <button class="ghost" onclick={() => open(() => Api.ShowInExplorer())}>Открыть папку</button>
      {#if plan?.siteButton}<button class="ghost" onclick={openSite}>{plan.siteButton}</button>{/if}
      <button class="primary" data-autofocus onclick={close}>Готово</button>
    </div>
  </Modal>
{/if}

<style>
  .memo {
    margin: 0 0 12px;
    padding: 0 0 0 18px;
    line-height: 1.6;
  }
  .site {
    margin: 0 0 12px;
    color: var(--muted);
    font-size: 12px;
  }
  .hint {
    margin: 0 0 12px;
    color: var(--muted);
    font-size: 12px;
  }
  .actions {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
    margin-top: 12px;
  }
</style>
