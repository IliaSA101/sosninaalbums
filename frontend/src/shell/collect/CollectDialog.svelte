<script lang="ts">
  import { untrack } from 'svelte'
  import { store } from '../../base/core/store.svelte'
  import { jobs } from '../../base/core/jobs.svelte'
  import Modal from '../../base/ui/Modal.svelte'
  import { collectUi } from './collect.svelte'
  import { extraLine, folderHint, missingLine, phaseOf, progressLine, progressPercent, summaryOf } from './collect'

  // Окно «Собрать проект в папку» (спека 069): что войдёт и объём, дальше — ход копирования.
  // Вид и порядок кнопок — как у «Экспорт»; вопросы, диалог папки и проверки ведёт Go.
  const phase = $derived(phaseOf(jobs.collectOpen, !!jobs.collecting))
  const info = $derived(collectUi.info)
  const missing = $derived(info ? missingLine(info) : '')
  const sum = $derived(summaryOf(collectUi.album, collectUi.full, info))
  const extra = $derived(info ? extraLine(info) : '')

  // Каждое открытие — заново: выбор из localStorage, прежние сводки и ошибка не показываются.
  $effect(() => {
    if (!jobs.collectOpen) return
    untrack(() => collectUi.reset())
  })
  // Сводка — при открытии и после каждой правки альбома (пока окно в настройках).
  $effect(() => {
    void store.view.rev
    if (phase !== 'setup') return
    untrack(() => void collectUi.refresh())
  })
  // Режим «Все»: лента ещё читает папки — числа неполные, спрашиваем снова.
  $effect(() => {
    if (phase !== 'setup' || !info?.scanning) return
    const t = setTimeout(() => void collectUi.refresh(), 1000)
    return () => clearTimeout(t)
  })
</script>

{#if phase !== 'closed'}
  <Modal title="Собрать проект в папку" onclose={() => collectUi.close()} width={560}>
    {#if phase === 'running' && jobs.collecting}
      {@const pct = progressPercent(jobs.collecting)}
      <div class="run">
        <progress max="100" value={pct ?? undefined} aria-label="Ход копирования"></progress>
        <div class="line" role="status">{progressLine(jobs.collecting)}</div>
        <p class="hint">Файл, который копировался в момент остановки, удаляется; скопированное остаётся. Альбом записывается в конце.</p>
        <div class="actions">
          <button data-autofocus onclick={() => collectUi.stop()}>Остановить</button>
        </div>
      </div>
    {:else}
      <div class="group" role="radiogroup" aria-label="Фото">
        <span class="cap">Фото</span>
        <div class="opts">
          <label
            ><input type="radio" name="col-all" checked={!collectUi.all} onchange={() => collectUi.setAll(false)} />Только фото альбома
            <span class="hint" class:pending={!collectUi.album}>{sum.albumHint}</span></label
          >
          <label
            ><input type="radio" name="col-all" checked={collectUi.all} onchange={() => collectUi.setAll(true)} />Все фото из папок альбома
            <span class="hint" class:pending={!collectUi.full}>{sum.fullHint}</span></label
          >
        </div>
      </div>
      {#if extra}<p class="hint">{extra}</p>{/if}
      {#if missing}<p class="warn">{missing}</p>{/if}
      <p class="hint">{folderHint(collectUi.folder)}</p>
      <p class="hint" class:pending={sum.pending}>{sum.total}</p>
      {#if collectUi.error}<p class="err" role="alert">{collectUi.error}</p>{/if}
      <div class="actions">
        <button class="ghost" onclick={() => collectUi.close()}>Отмена</button>
        <button class="primary" data-autofocus disabled={collectUi.busy} onclick={() => collectUi.go()}>Собрать…</button>
      </div>
    {/if}
  </Modal>
{/if}

<style>
  /* Во время копирования закрыть окно нельзя — крестик Modal не показываем (Esc и фон глушит collectUi.close) */
  :global(.box:has(.run) .x) {
    visibility: hidden;
  }
  /* Подпись строки — левая колонка 64 px, варианты — правая (как в «Экспорт») */
  .group {
    display: grid;
    grid-template-columns: 64px 1fr;
    align-items: start;
    gap: 0 14px;
    margin-bottom: 12px;
  }
  .opts {
    display: flex;
    flex-direction: column;
    gap: 6px;
    min-width: 0;
  }
  .cap {
    color: var(--muted);
    font-size: 12px;
    padding-top: 2px;
  }
  label {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    cursor: pointer;
  }
  input[type='radio'] {
    accent-color: var(--accent);
    margin: 0;
  }
  .hint {
    color: var(--muted);
    font-size: 12px;
    line-height: 1.45;
    margin: 0 0 8px;
  }
  label .hint {
    margin: 0;
  }
  /* Числа ещё не пришли: заглушка занимает то же место, но бледная */
  .pending {
    opacity: 0.45;
  }
  .warn {
    color: #e0803a;
    font-size: 12px;
    margin: 0 0 8px;
  }
  .err {
    color: #e5645a;
    font-size: 12px;
    margin: 0 0 8px;
  }
  progress {
    width: 100%;
    height: 8px;
    accent-color: var(--accent);
  }
  .line {
    font-size: 13px;
    margin: 10px 0 4px;
    font-variant-numeric: tabular-nums;
  }
  .actions {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
    margin-top: 14px;
  }
</style>
