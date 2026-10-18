<script lang="ts">
  import { store } from '../../base/core/store.svelte'
  import Modal from '../../base/ui/Modal.svelte'
  import Popover from '../../base/ui/Popover.svelte'
  import { sheetIn } from '../../base/geom/sheet'
  import { unitWords } from '../../editor/captions/unitword'
  import { vigDialog } from '../vignette/vigsession.svelte'
  import NamesNoSeats from './NamesNoSeats.svelte'
  import { namesUi } from './names.svelte'
  import { isPasteKey } from './names'
  import { columnsLine, EMPTY_SEAT, fileInfo, leftList, rowKey, summaryLine, vigHint, whoText } from './plantext'

  // «Имена» (спеки 030, 043): список (в том числе из файла Excel/CSV) или имена файлов → имена на фото листа.
  // Разбор и предпросмотр «фото ← строка» считает Go (PreviewNames, тот же план, что у вставки), здесь — показ.
  // Вставляет и ставит одну отмену Go (PasteNames).
  const SAMPLE = 'Иванов Пётр\nПетрова Анна Сергеевна\nСидоров Иван'
  const sp = $derived(sheetIn(store.view, namesUi.spreadId))
  const plan = $derived(namesUi.plan)
  const w = $derived(unitWords(store.view.settings, sp))
  const vig = $derived(!!plan?.rows.some((r) => r.tag))
  const nameOf = (id: string) => store.byId.get(id)?.name ?? id
  const err = $derived(namesUi.error || namesUi.planError)
  const canPaste = $derived(!!sp && namesUi.canPaste)
  const cols = $derived(columnsLine(plan))
  const left = $derived(plan ? leftList(plan) : { items: [], more: 0 })
  const hint = $derived(namesUi.from === 'list' ? vigHint(plan) : '')

  let helpOpen = $state(false)
  let helpBtn = $state<HTMLButtonElement>()

  // Предпросмотр заказывается при каждой смене входов; пауза и номер запроса — в namesUi.
  $effect(() => {
    namesUi.refresh()
  })

  // Образец привязан к кнопке списка: нет кнопки или окно закрыто — образец закрыт.
  $effect(() => {
    if (!namesUi.open || namesUi.from !== 'list') helpOpen = false
  })

  // Esc: сначала закрывается образец, потом окно.
  function close() {
    if (helpOpen) helpOpen = false
    else namesUi.close()
  }

  // Enter в поле — перенос строки, вставляет Ctrl+Enter. Слушаем на window: в режиме «Имена файлов» полей нет,
  // и после щелчка в пустое место фокус стоит на самом окне.
  let bodyEl = $state<HTMLElement>()
  function key(e: KeyboardEvent) {
    if (namesUi.open && canPaste && isPasteKey(e, bodyEl?.parentElement)) {
      e.preventDefault()
      void namesUi.apply()
    }
  }
</script>

<svelte:window onkeydown={key} />

{#if namesUi.open && sp}
  <Modal title="Имена" onclose={close} wide>
    <div class="body" bind:this={bodyEl}>
      <div class="seg" role="group" aria-label="Откуда имена">
        <button class:on={namesUi.from === 'list'} aria-pressed={namesUi.from === 'list'} onclick={() => (namesUi.from = 'list')}
          >Список</button
        >
        <button
          class:on={namesUi.from === 'files'}
          aria-pressed={namesUi.from === 'files'}
          onclick={() => (namesUi.from = 'files')}
          title="Имя берётся из названия фото">Имена файлов</button
        >
      </div>

      {#if namesUi.from === 'list'}
        <div class="head">
          <label class="cap" for="names-text">Список: по строке на человека, порядок — как фото {w.on}</label>
          <button class="ghost pick" disabled={namesUi.loading} onclick={() => namesUi.loadFile()} title="Excel (.xlsx), CSV или текст">
            Загрузить таблицу…
          </button>
          <button
            bind:this={helpBtn}
            class="help"
            type="button"
            aria-haspopup="dialog"
            aria-expanded={helpOpen}
            title="Как подготовить таблицу"
            aria-label="Образец таблицы"
            onclick={() => (helpOpen = !helpOpen)}>?</button
          >
        </div>
        {#if namesUi.file}<p class="info" title={namesUi.file.file}>{fileInfo(namesUi.file)}</p>{/if}
        <textarea id="names-text" data-names-text rows="7" spellcheck="false" data-autofocus placeholder={SAMPLE} bind:value={namesUi.text}
        ></textarea>
      {:else}
        <p class="cap">
          Имя берётся из названия фото {w.on}: «01_иванов_петя.jpg» → Иванов Петя. Номер в начале и «(2)» в конце не мешают, регистр
          исправляется.
        </p>
      {/if}

      <label class="chk" class:off={namesUi.orderOff}>
        <input type="checkbox" bind:checked={namesUi.firstFirst} disabled={namesUi.orderOff} />
        Имя первым («Пётр Иванов»)
        {#if namesUi.orderOff}<span class="note">Порядок задаёт заголовок</span>{/if}
      </label>

      {#if cols}<p class="cols">{cols}</p>{/if}

      <div class="cap">Куда ляжет</div>
      <ul class="rows" aria-label="Предпросмотр">
        {#each plan?.rows ?? [] as r (rowKey(r))}
          <li class:none={!r.who} class:vig>
            <span class="no">{r.no ? r.no : ''}</span>
            {#if vig}<span class="role" class:skip={!r.no}>{r.tag}</span>{/if}
            <span class="file" class:blank={!r.photo} title={r.photo ? nameOf(r.photo) : EMPTY_SEAT}
              >{r.photo ? nameOf(r.photo) : EMPTY_SEAT}</span
            >
            <span class="arrow" aria-hidden="true">←</span>
            <span class="who"
              >{whoText(r, vig)}{#if r.no && r.who?.subj}<span class="subj">{' · ' + r.who.subj}</span>{/if}</span
            >
          </li>
        {:else}
          {#if plan}<NamesNoSeats {sp} onCap={w.onCap} />{:else}<li class="none"></li>{/if}
        {/each}
      </ul>

      {#if left.items.length > 0}
        <div class="cap">Строки без фото</div>
        <ul class="leftrows" aria-label="Строки без фото">
          {#each left.items as l (l)}<li>{l}</li>{/each}
          {#if left.more > 0}<li class="more">…ещё {left.more}</li>{/if}
        </ul>
      {/if}

      {#if hint}
        <p class="vighint" role="status">
          {hint}
          <button class="ghost" onclick={() => vigDialog.open(sp.id)}>Настроить виньетку…</button>
        </p>
      {/if}

      {#if plan?.needCaption}
        <label class="chk">
          <input type="checkbox" bind:checked={namesUi.caption} />
          Подписать фото: {'{Фамилия} {Имя}'}
        </label>
      {/if}

      <p class="hint" role="status">
        {#if err}<span class="bad">{err}</span>{:else if plan}{summaryLine(plan)}{/if}
      </p>
      <div class="actions">
        <button class="ghost" onclick={() => namesUi.close()}>Отмена</button>
        <button class="primary" disabled={!canPaste} onclick={() => namesUi.apply()} title="Ctrl+Enter">Вставить</button>
      </div>
    </div>
  </Modal>
{/if}

<Popover anchor={helpBtn} bind:open={helpOpen} label="Образец таблицы" kind="names-help">
  <div class="sample">
    <table>
      <thead>
        <tr><th>Фамилия</th><th>Имя</th><th>Отчество</th><th>Предмет</th></tr>
      </thead>
      <tbody>
        <tr><td>Петрова</td><td>Анна</td><td>Сергеевна</td><td>Математика</td></tr>
        <tr><td>Иванов</td><td>Пётр</td><td>Игоревич</td><td></td></tr>
      </tbody>
    </table>
    <p>
      Первая строка — заголовки. Столбец «Предмет» (или «Должность») делает строку учителем, а сам предмет печатается, если в шаблоне
      подписи есть &#123;Предмет&#125;. Столбец «Файл» — по файлам. На Mac сохраняйте «CSV UTF-8».
    </p>
    <p>Одиночная кавычка " в начале ячейки, без закрывающей, склеит остаток файла.</p>
  </div>
</Popover>

<style>
  .body {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }
  .cap {
    font-size: 13px;
  }
  p.cap {
    margin: 0;
    color: var(--muted);
  }
  .head {
    display: flex;
    align-items: center;
    gap: 8px;
  }
  .head .cap {
    flex: 1;
    min-width: 0;
  }
  .pick {
    flex: none;
    height: 26px;
    font-size: 12px;
    padding: 0 10px;
  }
  /* «?» — как у строк матрицы уровней: круг мельче кнопки */
  .help {
    flex: none;
    width: 22px;
    height: 22px;
    padding: 0;
    border-radius: 50%;
    background: transparent;
    border: 1px solid var(--line);
    color: var(--muted);
    font-size: 12px;
    line-height: 1;
  }
  .help:hover:not(:disabled) {
    color: var(--accent);
    border-color: var(--accent);
  }
  .info,
  .cols,
  .note {
    margin: 0;
    color: var(--muted);
    font-size: 12px;
  }
  .info {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .seg {
    display: flex;
    gap: 1px;
    align-self: flex-start;
    padding: 2px;
    background: #1b1b1d;
    border-radius: 7px;
  }
  .seg button {
    height: 24px;
    padding: 0 12px;
    font-size: 12px;
    background: transparent;
    border-radius: 5px;
  }
  .seg button.on {
    background: var(--accent);
    color: #1e1e1e;
    font-weight: 600;
  }
  textarea {
    box-sizing: border-box;
    width: 100%;
    resize: vertical;
    font-family: inherit;
    font-size: 13px;
  }
  .chk {
    display: flex;
    align-items: center;
    gap: 6px;
  }
  .chk.off {
    color: var(--muted);
  }
  .rows {
    list-style: none;
    margin: 0;
    padding: 0;
    max-height: 180px;
    overflow-y: auto;
    border: 1px solid var(--line);
    border-radius: 6px;
    font-size: 12px;
  }
  .rows li {
    display: grid;
    grid-template-columns: 26px minmax(0, 1fr) 16px minmax(0, 1fr);
    align-items: center;
    gap: 6px;
    padding: 3px 8px;
  }
  .rows li.vig {
    grid-template-columns: 26px 74px minmax(0, 1fr) 16px minmax(0, 1fr);
  }
  .role {
    color: var(--muted);
    white-space: nowrap;
  }
  .role.skip {
    font-style: italic;
  }
  .rows li:nth-child(even) {
    background: rgba(255, 255, 255, 0.03);
  }
  .rows .none {
    color: var(--muted);
  }
  /* пустое место: фото нет — бледно и курсивом, как заглушка подписи */
  .rows .file.blank {
    color: var(--muted);
    font-style: italic;
  }
  .no {
    color: var(--muted);
    text-align: right;
    font-variant-numeric: tabular-nums;
  }
  .rows .file,
  .who {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .subj {
    color: var(--muted);
  }
  .arrow {
    color: var(--muted);
    text-align: center;
  }
  .leftrows {
    list-style: none;
    margin: 0;
    padding: 4px 8px;
    border: 1px solid var(--line);
    border-radius: 6px;
    font-size: 12px;
    color: var(--muted);
  }
  .leftrows .more {
    font-style: italic;
  }
  .vighint {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 8px;
    margin: 0;
    font-size: 12px;
  }
  .vighint button {
    height: 24px;
    font-size: 12px;
    padding: 0 10px;
  }
  .hint {
    margin: 0;
    min-height: 18px;
    color: var(--muted);
    font-size: 12px;
  }
  .bad {
    color: #e5645a;
  }
  .actions {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
    margin-top: 4px;
  }
  /* Образец в окне Popover (в body): стили этого компонента применяются, пока он смонтирован */
  .sample {
    width: 300px;
  }
  .sample table {
    border-collapse: collapse;
    width: 100%;
  }
  .sample th,
  .sample td {
    padding: 2px 6px;
    border: 1px solid var(--line);
    text-align: left;
    font-size: 11px;
  }
  .sample th {
    color: var(--muted);
    font-weight: 600;
  }
  .sample p {
    margin: 8px 0 0;
    color: var(--muted);
  }
</style>
