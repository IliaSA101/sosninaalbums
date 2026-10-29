<script lang="ts">
  import { updateUi } from './update.svelte'
  import { START_LABEL, PAGE_LABEL, WHATS_NEW_LABEL, updateView } from './updatetext'

  // Блок «Обновления» в «О программе» (111). Образцы: ход — «Наборы библиотеки», кнопки и ссылки — само окно «О программе».
  // «Что нового» — строки из подписанного файла выпуска: только текстом, не {@html}.
  const v = $derived(updateView(updateUi.info, updateUi.retry))
  const disabled = (d?: boolean) => d || updateUi.cancelling
</script>

<div class="upd" role="group" aria-label="Обновления">
  <div class="title">Обновления</div>
  {#if v.line}
    <p class="state" class:err={updateUi.info.phase === 'error'} role="status">{updateUi.cancelling ? 'Отменяем…' : v.line}</p>
  {/if}
  {#if v.pct !== null || v.busy}
    <progress max="100" value={v.pct ?? undefined} aria-label="Ход скачивания обновления"></progress>
  {/if}
  {#if v.notes.length > 0}
    <button class="link" aria-expanded={updateUi.notesOpen} onclick={() => (updateUi.notesOpen = !updateUi.notesOpen)}
      >{WHATS_NEW_LABEL} {updateUi.notesOpen ? '▾' : '▸'}</button
    >
    {#if updateUi.notesOpen}
      <ul class="notes" aria-label="Что нового в версии {updateUi.info.version}">
        {#each v.notes as n, i (i)}<li>{n}</li>{/each}
      </ul>
    {/if}
  {/if}
  {#if v.hint}<p class="muted">{v.hint}</p>{/if}
  <div class="row">
    {#each v.btns as b (b.act + b.label)}
      <button class={b.primary ? 'primary' : 'ghost'} disabled={disabled(b.disabled)} onclick={() => void updateUi.act(b.act)}
        >{b.label}</button
      >
    {/each}
    {#if v.pageLink}
      <button class="link" onclick={() => void updateUi.openPage()}>{PAGE_LABEL}</button>
    {/if}
  </div>
  <label class="chk">
    <input type="checkbox" checked={updateUi.info.atStart} onchange={(e) => void updateUi.setAtStart(e.currentTarget.checked)} />
    {START_LABEL}
  </label>
</div>

<style>
  .upd {
    margin-top: 12px;
    padding-top: 10px;
    border-top: 1px solid var(--line);
  }
  .title {
    font-weight: 600;
    margin-bottom: 6px;
  }
  .state {
    margin: 0 0 6px;
    line-height: 1.4;
    user-select: text;
    font-variant-numeric: tabular-nums;
  }
  .err {
    color: #ff8a8a;
  }
  .muted {
    margin: 0 0 6px;
    color: var(--muted);
    font-size: 12px;
    line-height: 1.4;
  }
  progress {
    display: block;
    width: 100%;
    height: 6px;
    margin: 0 0 8px;
    accent-color: var(--accent);
  }
  .notes {
    margin: 4px 0 6px;
    padding-left: 18px;
    color: var(--muted);
    font-size: 12px;
    line-height: 1.4;
    overflow-wrap: anywhere;
    user-select: text;
    /* Длинный список не выталкивает кнопки окна за низ (1024×700): свой скролл */
    max-height: min(68px, 10vh);
    overflow-y: auto;
  }
  .row {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 8px;
    margin: 6px 0;
  }
  .chk {
    display: flex;
    align-items: center;
    gap: 6px;
    margin-top: 4px;
  }
  /* Ссылка-кнопка: как в окне наборов */
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
</style>
