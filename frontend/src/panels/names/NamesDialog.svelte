<script lang="ts">
  import { store } from '../../base/core/store.svelte'
  import Modal from '../../base/ui/Modal.svelte'
  import { sheetIn } from '../../base/geom/sheet'
  import { unitWords } from '../../editor/captions/unitword'
  import { namesUi } from './names.svelte'
  import { parseNames, personLine, previewSlots, slotsOf, summary } from './names'

  // «Имена из списка» (спека 030): вставьте столбец из Excel или Word — строка «Фамилия Имя [Отчество]» на фото
  // листа по порядку. Предпросмотр «фото ← строка» и итог внизу считаются здесь по тем же правилам, что в Go;
  // вставляет и ставит одну отмену Go (PasteNames).
  const SAMPLE = 'Иванов Пётр\nПетрова Анна Сергеевна\nСидоров Иван'
  const sp = $derived(sheetIn(store.view, namesUi.spreadId))
  const parsed = $derived(parseNames(namesUi.text, namesUi.firstFirst))
  const view = $derived(sp ? previewSlots(slotsOf(sp), parsed.people, (id) => !!namesUi.named[id]) : null)
  const w = $derived(unitWords(store.view.settings, sp))
  const vig = $derived(!!sp?.vig)
  const nameOf = (id: string) => store.byId.get(id)?.name ?? id
  const err = $derived(parsed.error || namesUi.error)
  const canPaste = $derived(!!sp && !namesUi.busy && !parsed.error && parsed.people.length > 0)

  // Enter в поле — перенос строки, вставляет Ctrl+Enter.
  function key(e: KeyboardEvent) {
    if (e.key === 'Enter' && (e.ctrlKey || e.metaKey) && canPaste) {
      e.preventDefault()
      void namesUi.apply()
    }
  }
</script>

{#if namesUi.open && sp}
  <Modal title="Имена из списка" onclose={() => namesUi.close()} wide>
    <div class="body" role="presentation" onkeydown={key}>
      <label class="cap" for="names-text">Список: по строке на человека, порядок — как фото {w.on}</label>
      <textarea id="names-text" rows="7" spellcheck="false" data-autofocus placeholder={SAMPLE} bind:value={namesUi.text}></textarea>
      <label class="chk">
        <input type="checkbox" bind:checked={namesUi.firstFirst} />
        Имя первым («Пётр Иванов»)
      </label>

      <div class="cap">Куда ляжет</div>
      <ul class="rows" aria-label="Предпросмотр">
        {#each view?.slots ?? [] as r (r.photo)}
          <li class:none={!r.line} class:vig>
            <span class="no">{r.skipped ? '' : r.no}</span>
            {#if vig}<span class="role" class:skip={r.skipped}>{r.tag}</span>{/if}
            <span class="file" title={nameOf(r.photo)}>{nameOf(r.photo)}</span>
            <span class="arrow" aria-hidden="true">←</span>
            <span class="who">{r.skipped ? 'без имени' : r.line ? personLine(r.line) : '—'}</span>
          </li>
        {:else}
          <li class="none">{w.onCap} нет фото</li>
        {/each}
      </ul>

      <p class="hint" role="status">
        {#if err}<span class="bad">{err}</span>{:else}{view ? summary(view) : ''}{/if}
      </p>
      <div class="actions">
        <button class="ghost" onclick={() => namesUi.close()}>Отмена</button>
        <button class="primary" disabled={!canPaste} onclick={() => namesUi.apply()} title="Ctrl+Enter">Вставить</button>
      </div>
    </div>
  </Modal>
{/if}

<style>
  .body {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }
  .cap {
    font-size: 13px;
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
  .no {
    color: var(--muted);
    text-align: right;
    font-variant-numeric: tabular-nums;
  }
  .file,
  .who {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .arrow {
    color: var(--muted);
    text-align: center;
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
</style>
