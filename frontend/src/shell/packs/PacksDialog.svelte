<script lang="ts">
  import { EventsOn } from '../../../wailsjs/runtime/runtime'
  import Modal from '../../base/modal/Modal.svelte'
  import { READ_ONLY_HINT } from '../../panels/decor/decor'
  import { packsUi } from './packs.svelte'
  import {
    ADD_HINT,
    AUTHORS_LINK,
    AUTHORS_TEXT,
    BUILTIN_NOTE,
    BUSY_HINT,
    EMPTY_TEXT,
    creditLine,
    hasNet,
    metaLine,
    onlyBuiltin,
  } from './packs'
  import { aboutUi } from '../dialogs/about.svelte'
  import {
    CONSENT_TEXT,
    CONSENT_TITLE,
    REVOKE_HINT,
    START_LABEL,
    START_LABEL_ASK,
    SKIP_RESET_HINT,
    SKIP_RESET_LABEL,
    skipLine,
    progressPercent,
    progressText,
    type LibraryChanged,
    type LibraryOffer,
    type LibraryProgress,
    type LibraryUpdate,
  } from './packnet'

  // Окно «Наборы библиотеки» (067 Ф2): по образцу «Резервных копий» — список, кнопка у строки, «Закрыть» внизу.
  // Название, автор, лицензия, источник и описание — недоверенный текст из манифеста набора: только как текст (не {@html}).
  // Сеть (Ф3): «Обновить библиотеку» с полосой хода (образец — экспорт и сбор в папку), вопрос согласия, флажок и «Не обращаться к сети».
  const net = packsUi.net

  // События Go (компонент смонтирован всегда, окно может быть закрыто — тост предложения и ход нужны и тогда):
  // library:changed — список и каталог открытого окна «Фоны/Стикеры/Развороты» перечитываются;
  // library:updated — итог проверки при запуске: обычный тост, как у кнопки «Обновить библиотеку» (073-2).
  $effect(() => {
    const off = [
      EventsOn('library:changed', (r?: LibraryChanged) => packsUi.changed(r?.installed)),
      EventsOn('library:updated', (r: LibraryUpdate) => net.updated(r)),
      EventsOn('library:progress', (p: LibraryProgress) => net.progressed(p)),
      EventsOn('library:offer', (o: LibraryOffer) => net.offered(o)),
    ]
    return () => off.forEach((f) => f?.())
  })

  const lockHint = $derived(packsUi.readOnly ? READ_ONLY_HINT : packsUi.goBusy || net.updating ? BUSY_HINT : '')
  const locked = $derived(packsUi.locked || packsUi.readOnly)
  const working = $derived(net.updating || !!net.progress)
  const pct = $derived(progressPercent(net.progress))
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
              <!-- Полный текст в title: в строке он обрезан line-clamp: 2 (design 129 §6) -->
              {#if p.about}<span class="meta about" title={p.about}>{p.about}</span>{/if}
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
    {#if hasNet(packsUi.packs)}
      <!-- Подпись каталога (129 §6): только если стоит набор из сети; ссылка — как в «О программе» -->
      <p class="skip">
        {AUTHORS_TEXT} ·
        <button class="link" onclick={() => void aboutUi.link('authors')}>{AUTHORS_LINK}</button>
      </p>
    {/if}
    {#if net.net.skipped > 0}
      <!-- Удалённые сетевые наборы в пропуске (решение 0.17 №3): видно, почему набора нет, и возврат одним щелчком -->
      <p class="skip">
        {skipLine(net.net.skipped)} ·
        <button class="link" disabled={locked} title={lockHint || SKIP_RESET_HINT} onclick={() => net.resetSkip()}
          >{SKIP_RESET_LABEL}</button
        >
      </p>
    {/if}
    {#if packsUi.error}<p class="err" role="alert">{packsUi.error}</p>{/if}
    {#if working}
      <!-- Ход обновления (образец — экспорт): строка «Скачиваем «…» — 12 из 40 МБ», полоса, «Отменить» -->
      <div class="progress">
        <div class="row">
          <span role="status">{net.cancelling ? 'Отменяем…' : progressText(net.progress)}</span>
          <span class="grow"></span>
          <button class="ghost" disabled={net.cancelling} onclick={() => net.cancel()}>Отменить</button>
        </div>
        <progress max="100" value={pct ?? undefined} aria-label="Ход обновления библиотеки"></progress>
      </div>
    {/if}
    <div class="opts">
      <button disabled={locked} title={lockHint} onclick={() => net.update()}>Обновить библиотеку</button>
      <label class="chk">
        <input
          type="checkbox"
          checked={net.net.atStart}
          onchange={(e) => {
            // Состояние флажка — у Go: до ответа (или вопроса согласия) вид прежний
            const on = e.currentTarget.checked
            e.currentTarget.checked = net.net.atStart
            void net.setStart(on)
          }}
        />
        {START_LABEL}
      </label>
      <span class="grow"></span>
      {#if net.net.allowed}
        <button class="link" title={REVOKE_HINT} onclick={() => net.revoke()}>Не обращаться к сети</button>
      {/if}
    </div>
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

{#if packsUi.open && net.consent}
  <!-- Первое «Обновить библиотеку» (сц. 12): без согласия программа в сеть не ходит. Порядок кнопок — «Отмена» слева, действие справа -->
  <Modal title={CONSENT_TITLE} onclose={() => net.dismissConsent()} wide>
    <p>{CONSENT_TEXT}</p>
    <label class="chk"><input type="checkbox" bind:checked={net.consentStart} />{START_LABEL_ASK}</label>
    <div class="actions">
      <button class="ghost" onclick={() => net.dismissConsent()}>Отмена</button>
      <button class="primary" data-autofocus onclick={() => net.confirmConsent()}>Разрешить и обновить</button>
    </div>
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
  .progress {
    margin: 0 0 10px;
    padding: 8px 10px;
    border: 1px solid var(--line);
    border-radius: 6px;
    background: var(--panel2);
  }
  .row {
    display: flex;
    align-items: center;
    gap: 10px;
    margin-bottom: 6px;
    font-variant-numeric: tabular-nums;
  }
  .grow {
    flex: 1;
  }
  progress {
    width: 100%;
    height: 6px;
    accent-color: var(--accent);
  }
  .opts {
    display: flex;
    align-items: center;
    gap: 12px;
    margin: 0 0 10px;
  }
  .chk {
    display: flex;
    align-items: center;
    gap: 6px;
    margin: 0 0 10px;
  }
  .opts .chk {
    margin: 0;
  }
  /* Ссылка-кнопка: как «Править» в сведениях об альбоме */
  .link {
    all: unset;
    cursor: pointer;
    color: var(--muted);
    font-size: 12px;
    text-decoration: underline dotted;
    text-underline-offset: 3px;
  }
  .link:hover,
  .link:focus-visible {
    text-decoration-style: solid;
  }
  .link:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: 2px;
  }
  .link:disabled {
    cursor: default;
    opacity: 0.5;
    text-decoration: none;
  }
  .skip {
    margin: 0 0 10px;
    color: var(--muted);
    font-size: 12px;
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
