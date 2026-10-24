<script lang="ts">
  import Modal from '../../base/modal/Modal.svelte'
  import PrintProfileForm from './PrintProfileForm.svelte'
  import { printEdit as ui } from './printedit.svelte'
  import { printList } from './printlist.svelte'
  import { IMPORT_HINT, NOT_OWN_HINT, SAVE_BAD_HINT, WINDOW_TITLE, removeHint, shareHint } from './printwords'

  // Окно «Своя типография» (123 Б3). Смонтировано всегда (как «Наборы библиотеки»): список типографий для меню и настроек
  // живёт и при закрытом окне — print:changed обновляет его. «Сохранить» создаёт новую типографию и ставит её альбому;
  // «Поделиться» и «Удалить» работают с типографией, выбранной в «Основе», и только если она своя.
  $effect(() => {
    const off = printList.watch()
    return () => off()
  })
</script>

{#if ui.open}
  <Modal title={WINDOW_TITLE} onclose={() => ui.close()} width={860}>
    <PrintProfileForm />
    <footer>
      <div class="l">
        <button class="ghost" type="button" disabled={ui.busy} title={IMPORT_HINT} onclick={() => ui.importFile()}
          >Добавить из файла…</button
        >
        <button
          class="ghost"
          type="button"
          disabled={!ui.ownBase || ui.busy}
          title={ui.ownBase ? shareHint(ui.baseName) : NOT_OWN_HINT}
          onclick={() => ui.share()}>Поделиться файлом…</button
        >
        <button
          class="ghost"
          type="button"
          disabled={!ui.ownBase || ui.busy}
          title={ui.ownBase ? removeHint(ui.baseName) : NOT_OWN_HINT}
          onclick={() => ui.askRemove()}>Удалить</button
        >
      </div>
      <div class="r">
        <button class="ghost" type="button" onclick={() => ui.close()}>Отмена</button>
        <button class="primary" type="button" disabled={!ui.canSave} title={ui.check?.ok ? '' : SAVE_BAD_HINT} onclick={() => ui.save()}
          >Сохранить</button
        >
      </div>
    </footer>
  </Modal>
{/if}

{#if ui.open && ui.ask}
  <!-- Вопросы поверх окна: «Отмена» слева, действие справа (как в «Наборах библиотеки») -->
  <Modal title={ui.ask.kind === 'replace' ? 'Заменить типографию?' : 'Удалить типографию?'} onclose={() => ui.dismiss()} wide>
    <p>{ui.ask.text}</p>
    {#if ui.ask.note}<p class="meta">{ui.ask.note}</p>{/if}
    <div class="actions">
      <button class="ghost" type="button" onclick={() => ui.dismiss()}>Отмена</button>
      {#if ui.ask.kind === 'replace'}
        <button class="primary" type="button" data-autofocus onclick={() => ui.confirmReplace()}>Заменить</button>
      {:else}
        <button class="primary" type="button" data-autofocus onclick={() => ui.confirmRemove()}>Удалить</button>
      {/if}
    </div>
  </Modal>
{/if}

<style>
  footer {
    display: flex;
    justify-content: space-between;
    gap: 8px;
    flex-wrap: wrap;
  }
  .l,
  .r {
    display: flex;
    gap: 8px;
    flex-wrap: wrap;
  }
  p {
    margin: 0 0 10px;
    line-height: 1.4;
  }
  .meta {
    color: var(--muted);
    font-size: 12px;
    overflow-wrap: anywhere;
  }
  .actions {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
    margin-top: 12px;
  }
</style>
