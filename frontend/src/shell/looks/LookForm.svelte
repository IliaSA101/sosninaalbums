<script lang="ts">
  import { store } from '../../base/core/store.svelte'
  import Seg from '../../base/tpl/Seg.svelte'
  import { isSheet, unitOf } from '../../base/geom/sheet'
  import { BG_NOTE, MODE_ITEMS, SWAP_NOTE, fitText, formatText, placesText, stepLabel, stepWhy } from './looks'
  import { looks } from './looks.svelte'

  // Низ правой колонки: «Что сделать» (только в собранном альбоме), «Разворотов», подбор, формат и кнопки.
  // Режим и числа даёт Go (PlanLook / LookCount); здесь поля и слова. В «Добавить» число — сколько повторяемых добавить,
  // места — из плана добавления, подбора нет (подбор считает серию целиком).
  const plan = $derived(looks.plan)
  const mode = $derived(looks.mode)
  const k = $derived(looks.sel ? looks.photos() : 0)
  const fit = $derived(looks.cnt?.fit ?? 0)

  // «Поставить на разворот N» (сц. 7): показанный шаг на текущий лист. Серая, если шаг не для этого листа: обложка и разворот не меняются.
  const target = $derived(looks.stepTarget)
  const why = $derived(target ? stepWhy(looks.step, target.cover, !!store.cover, !!looks.info?.cover) : '')
  const sheetDoc = $derived(isSheet(store.view.settings)) // у проекта-листа 031 макетов альбома нет — кнопки тоже

  // Enter в поле числа применяет значение, как blur.
  async function commit(e: Event) {
    const el = e.currentTarget as HTMLInputElement
    await looks.setSpreads(el.valueAsNumber)
    el.value = String(looks.shownN) // число вне пределов или пусто — в поле остаётся принятое
  }
  function enter(e: KeyboardEvent) {
    if (e.key === 'Enter') {
      e.preventDefault()
      void commit(e)
    }
  }
</script>

<div class="form">
  {#if plan?.full}
    <div class="row">
      <span class="lbl">Что сделать</span>
      <Seg items={MODE_ITEMS} value={looks.pick} onpick={(m) => looks.setPick(m)} label="Что сделать" />
    </div>
    {#if mode === 'bg'}<p class="note">{BG_NOTE}</p>{/if}
  {:else if plan?.prev}
    <p class="note">{SWAP_NOTE}</p>
  {/if}

  {#if looks.info && mode !== 'bg'}
    <div class="row">
      <label class="lbl" for="look-spreads">Разворотов</label>
      <input
        id="look-spreads"
        type="number"
        min={mode === 'add' ? 1 : looks.info.min}
        max={looks.info.max}
        value={looks.shownN}
        onchange={commit}
        onkeydown={enter}
      />
      <span class="places">{placesText(looks.shownPlaces)}</span>
    </div>
    {#if mode !== 'add' && k > 0 && fit > 0}
      <div class="row">
        <button type="button" class="link" onclick={() => looks.fit()}>{fitText(k, looks.fromSelection)}</button>
      </div>
    {/if}
  {/if}

  <div class="fmt">
    Формат альбома: {formatText(store.view.settings)} ·
    <button type="button" class="link" title="Настройки альбома в панели деталей" onclick={() => looks.openFormat()}>Изменить…</button>
  </div>

  {#if target && !sheetDoc}
    <button type="button" class="ghost step" disabled={!looks.stepOk} title={why || undefined} onclick={() => looks.askStep()}>
      {stepLabel(target.cover, target.no, unitOf(store.view.settings).one)}
    </button>
  {/if}

  <div class="actions">
    <button class="ghost" onclick={() => looks.close()}>Отмена</button>
    <button class="primary" disabled={!looks.ready || looks.busy} onclick={() => looks.apply()}>Применить</button>
  </div>
</div>

<style>
  .form {
    flex: none;
    display: flex;
    flex-direction: column;
    gap: 8px;
    margin-top: auto; /* форма прижата к низу колонки, предпросмотр сверху */
  }
  .row {
    display: flex;
    align-items: center;
    gap: 8px;
  }
  .lbl {
    color: var(--muted);
    font-size: 12px;
  }
  input[type='number'] {
    width: 72px;
  }
  .places {
    color: var(--muted);
    font-size: 12px;
  }
  .note {
    margin: 0;
    color: var(--muted);
    font-size: 12px;
    line-height: 1.4;
  }
  .fmt {
    color: var(--muted);
    font-size: 12px;
  }
  .link {
    all: unset;
    color: var(--accent);
    cursor: pointer;
    text-decoration: underline;
    font-size: 12px;
  }
  .link:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: 1px;
  }
  .step {
    align-self: flex-start;
  }
  .actions {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
  }
</style>
