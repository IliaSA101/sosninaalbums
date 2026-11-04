<script lang="ts">
  import { EventsOn } from '../../../wailsjs/runtime/runtime'
  import Modal from '../../base/modal/Modal.svelte'
  import { READ_ONLY_HINT } from '../../panels/decor/decor'
  import { packsUi } from './packs.svelte'
  import PackRow from './PackRow.svelte'
  import PacksAvail from './PacksAvail.svelte'
  import NetConsent from './NetConsent.svelte'
  import PackRun from './PackRun.svelte'
  import { ADD_HINT, AUTHORS_LINK, AUTHORS_TEXT, BUILTIN_NOTE, BUSY_HINT, creditLine, hasNet, metaLine } from './packs'
  import { aboutUi } from '../dialogs/about.svelte'
  import {
    consentLabel,
    REVOKE_HINT,
    START_LABEL,
    SKIP_RESET_LABEL,
    skipHint,
    skipText,
    type LibraryChanged,
    type LibraryProgress,
    type LibraryStart,
  } from './packnet'

  // Окно «Наборы библиотеки» (067 Ф2): по образцу «Резервных копий» — список, кнопка у строки, «Закрыть» внизу.
  // Название, автор, лицензия, источник и описание — недоверенный текст из манифеста набора: только как текст (не {@html}).
  // Сеть (Ф3): «Обновить библиотеку» с полосой хода (образец — экспорт и сбор в папку), вопрос согласия, флажок и «Не обращаться к сети».
  const net = packsUi.net

  // События Go (компонент смонтирован всегда, окно может быть закрыто — тост предложения и ход нужны и тогда):
  // library:changed — список и каталог открытого окна «Фоны/Стикеры/Развороты» перечитываются;
  // library:start — итог проверки при запуске одним тостом: обновлено, есть обновления, есть новые наборы (137-16).
  $effect(() => {
    const off = [
      EventsOn('library:changed', (r?: LibraryChanged) => packsUi.changed(r?.installed)),
      EventsOn('library:start', (s: LibraryStart) => net.started(s)),
      EventsOn('library:progress', (p: LibraryProgress) => net.progressed(p)),
      EventsOn('library:previews', () => packsUi.previews()), // 144: образцы докачаны — каталоги открытых окон перечитываются
    ]
    return () => off.forEach((f) => f?.())
  })

  const lockHint = $derived(packsUi.readOnly ? READ_ONLY_HINT : packsUi.goBusy || net.updating ? BUSY_HINT : '')
  const locked = $derived(packsUi.locked || packsUi.readOnly)
</script>

{#if packsUi.open}
  <Modal title="Наборы библиотеки" onclose={() => packsUi.close()} width={620}>
    <!-- Одна область прокрутки на оба раздела (137-26): «Обновить», ход и кнопки окна — вне неё, всегда на виду -->
    <div class="wrap">
      <div class="scroll">
        {#if packsUi.loading}
          <p class="note">Читаем список…</p>
        {:else}
          <ul class="list" aria-label="Наборы">
            {#each packsUi.packs as p (p.id)}
              <PackRow title={p.title} lines={[metaLine(p), creditLine(p)]} about={p.about}>
                {#if p.removable}
                  <button disabled={locked} title={lockHint || `Удалить набор «${p.title}»`} onclick={() => packsUi.askRemove(p)}
                    >Удалить</button
                  >
                {:else}
                  <span class="badge">{BUILTIN_NOTE}</span>
                {/if}
              </PackRow>
            {/each}
          </ul>
        {/if}
        {#if hasNet(packsUi.packs)}
          <!-- Подпись каталога (129 §6): только если стоит набор из сети; ссылка — как в «О программе» -->
          <p class="skip">
            {AUTHORS_TEXT} ·
            <button class="link" onclick={() => void aboutUi.link('authors')}>{AUTHORS_LINK}</button>
          </p>
        {/if}
        {#if !packsUi.loading}<PacksAvail {locked} {lockHint} />{/if}
        {#if net.net.skipped > 0}
          <!-- Удалённые вами наборы в «Можно скачать» помечены; «Скачать все» их пропускает, возврат — одним щелчком -->
          <p class="skip">
            {skipText(net.net)} ·
            <button class="link" disabled={locked} title={lockHint || skipHint(net.net)} onclick={() => net.resetSkip()}
              >{SKIP_RESET_LABEL}</button
            >
          </p>
        {/if}
      </div>
      {#if packsUi.error}<p class="err" role="alert">{packsUi.error}</p>{/if}
      <!-- Ход обновления (образец — экспорт): строка «Скачиваем «…» — 12 из 40 МБ», полоса, «Отменить» -->
      <PackRun {net} />
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
    </div>
  </Modal>
{/if}

{#if packsUi.open && net.consent}
  <!-- Первое сетевое действие («Обновить», «Скачать», «Скачать все»): без согласия программа в сеть не ходит -->
  <NetConsent
    action={consentLabel(net.pending)}
    bind:start={net.consentStart}
    onconfirm={() => net.confirmConsent()}
    oncancel={() => net.dismissConsent()}
  />
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
  /* Колонка окна: прокручивается только список (оба раздела), кнопки и ход — на виду. 100px — шапка и поля Modal */
  .wrap {
    display: flex;
    flex-direction: column;
    max-height: calc(100vh - 100px);
  }
  .wrap > :not(.scroll) {
    flex: none;
  }
  .scroll {
    flex: 1 1 auto;
    min-height: 0;
    overflow: auto;
    margin: 0 0 10px;
  }
  .list {
    list-style: none;
    margin: 0 0 12px;
    padding: 0;
    border: 1px solid var(--line);
    border-radius: 6px;
  }
  .meta {
    color: var(--muted);
    font-size: 12px;
    overflow-wrap: anywhere;
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
  .grow {
    flex: 1;
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
