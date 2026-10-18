<script lang="ts">
  import { EventsOn } from '../../../wailsjs/runtime/runtime'
  import Modal from '../../base/ui/Modal.svelte'
  import { READ_ONLY_HINT } from '../../panels/decor/decor'
  import { packsUi } from './packs.svelte'
  import { ADD_HINT, BUILTIN_NOTE, BUSY_HINT, EMPTY_TEXT, creditLine, metaLine, onlyBuiltin } from './packs'

  // Окно «Наборы библиотеки» (067 Ф2): по образцу «Резервных копий» — список, кнопка у строки, «Закрыть» внизу.
  // Название, автор, лицензия, источник и описание — недоверенный текст из манифеста набора: только как текст (не {@html}).
  // Сеть («Обновить библиотеку») — Ф3, в 0.17.

  // Go сообщает, что библиотека изменилась: список и каталог открытого окна «Фоны/Стикеры/Развороты» перечитываются.
  $effect(() => EventsOn('library:changed', () => packsUi.changed()))

  const lockHint = $derived(packsUi.readOnly ? READ_ONLY_HINT : packsUi.goBusy ? BUSY_HINT : '')
  const locked = $derived(packsUi.locked || packsUi.readOnly)
</script>

{#if packsUi.open}
  <Modal title="Наборы библиотеки" onclose={() => packsUi.close()} width={620}>
    {#if packsUi.loading}
      <p class="note">Читаем список…</p>
    {:else}
      <ul class="list" aria-label="Наборы">
        {#each packsUi.packs as p (p.id)}
          <li>
            <div class="info">
              <span class="when">{p.title}</span>
              {#if metaLine(p)}<span class="meta">{metaLine(p)}</span>{/if}
              {#if creditLine(p)}<span class="meta">{creditLine(p)}</span>{/if}
              {#if p.about}<span class="meta about">{p.about}</span>{/if}
            </div>
            {#if p.removable}
              <button disabled={locked} title={lockHint || `Удалить набор «${p.title}»`} onclick={() => packsUi.askRemove(p)}
                >Удалить</button
              >
            {:else}
              <span class="badge">{BUILTIN_NOTE}</span>
            {/if}
          </li>
        {/each}
      </ul>
      {#if onlyBuiltin(packsUi.packs) && !packsUi.error}<p class="note">{EMPTY_TEXT}</p>{/if}
    {/if}
    {#if packsUi.error}<p class="err" role="alert">{packsUi.error}</p>{/if}
    <footer>
      <div class="l">
        <button class="ghost" disabled={locked} title={lockHint || ADD_HINT} onclick={() => packsUi.addFile()}>Добавить из файла…</button>
      </div>
      <div class="r">
        <button class="ghost" title="Открыть папку библиотеки в проводнике" onclick={() => packsUi.showFolder()}>Показать папку</button>
        <button data-autofocus onclick={() => packsUi.close()}>Закрыть</button>
      </div>
    </footer>
  </Modal>
{/if}

{#if packsUi.open && packsUi.ask?.kind === 'remove'}
  <!-- Числа от Go: что освободится и что останется (нужно открытому альбому, своим, другим наборам) -->
  <Modal title="Удалить набор?" onclose={() => packsUi.dismiss()} wide>
    <p>{packsUi.ask.text}</p>
    <p class="meta">{packsUi.ask.note}</p>
    <div class="actions">
      <button class="ghost" onclick={() => packsUi.dismiss()}>Отмена</button>
      <button class="primary" data-autofocus onclick={() => packsUi.confirmRemove()}>Удалить</button>
    </div>
  </Modal>
{:else if packsUi.open && packsUi.ask?.kind === 'replace'}
  <Modal title="Заменить набор?" onclose={() => packsUi.dismiss()} wide>
    <p>{packsUi.ask.text}</p>
    <div class="actions">
      <button class="ghost" onclick={() => packsUi.dismiss()}>Отмена</button>
      <button class="primary" data-autofocus onclick={() => packsUi.confirmReplace()}>Заменить</button>
    </div>
  </Modal>
{/if}

<style>
  .note {
    margin: 0 0 12px;
    color: var(--muted);
  }
  .list {
    list-style: none;
    margin: 0 0 12px;
    padding: 0;
    max-height: 320px;
    overflow: auto;
    border: 1px solid var(--line);
    border-radius: 6px;
  }
  li {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    padding: 6px 10px;
  }
  li + li {
    border-top: 1px solid var(--line);
  }
  .info {
    display: flex;
    flex-direction: column;
    min-width: 0;
  }
  .when {
    font-weight: 600;
    overflow-wrap: anywhere;
  }
  .meta {
    color: var(--muted);
    font-size: 12px;
    overflow-wrap: anywhere;
  }
  /* Описание из манифеста длинное: показываем две строки */
  .about {
    display: -webkit-box;
    -webkit-line-clamp: 2;
    line-clamp: 2;
    -webkit-box-orient: vertical;
    overflow: hidden;
  }
  .badge {
    flex: none;
    color: var(--muted);
    font-size: 12px;
  }
  .err {
    margin: 0 0 12px;
    color: #ff8a8a;
  }
  p {
    margin: 0 0 10px;
    line-height: 1.4;
  }
  footer {
    display: flex;
    justify-content: space-between;
    gap: 8px;
  }
  .l,
  .r {
    display: flex;
    gap: 8px;
  }
  .actions {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
    margin-top: 12px;
  }
</style>
