<script lang="ts">
  import { Api, store } from '../../base/core/store.svelte'
  import { jobs } from '../../base/core/jobs.svelte'
  import { errText } from '../../base/words/errtext'
  import Modal from '../../base/ui/Modal.svelte'
  import { doneFolder } from './exportopts'

  // Окно «Готово для Фотосферы» (спека 047, решение 57): памятка нужна, пока фотограф заполняет калькулятор на сайте,
  // а тост гаснет через 8 с. Открыто, пока jobs.exportMemo не null; строки приходят из Go (export:done).
  const memo = $derived(jobs.exportMemo ?? [])
  // Имя папки — из плана, запомненного при запуске выгрузки (в памятке оно через « — » и не разбирается).
  const folder = $derived(doneFolder(jobs.exportFolder))

  function close() {
    jobs.exportMemo = null
    jobs.exportNote = ''
    jobs.exportFolder = ''
  }
  // Окно остаётся открытым: фотограф идёт на сайт и возвращается к памятке.
  async function open(call: () => Promise<void>) {
    try {
      await call()
    } catch (e) {
      store.notify(errText(e), 'error')
    }
  }
</script>

{#if jobs.exportMemo}
  <Modal title="Готово для Фотосферы" onclose={close} wide>
    <ul class="memo">
      {#each memo as line (line)}<li>{line}</li>{/each}
    </ul>
    <p class="site">На сайте: калькулятор Pro → эти параметры → «Загрузить» → файлы папки «{folder}».</p>
    {#if jobs.exportNote}<p class="hint">{jobs.exportNote}.</p>{/if}

    <div class="actions">
      <button class="ghost" onclick={() => open(() => Api.ShowInExplorer())}>Открыть папку</button>
      <button class="ghost" onclick={() => open(() => Api.OpenFsfera())}>Открыть Фотосферу</button>
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
