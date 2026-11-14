<script lang="ts">
  import { prefs, type LevelState } from '../../base/prefs/prefs.svelte'
  import { LEVELS, LEVEL_TITLE, can, featureGroups, normBase, type Base, type Level, type RowId } from '../../base/prefs/levels'
  import { fromBase, locked, rows } from '../../base/prefs/levelcustom'
  import { guide } from '../guide/guidestate.svelte'
  import { resetText, rowTip, togetherText } from './matrix'

  // Матрица функций (спека 011, п. 2, 9–13): функции строками по группам, три готовых уровня с ✓ и колонка
  // «Свой» с галочками. Правка идёт через prefs; под таблицей — строка статуса с «Вернуть».
  // pick — нижняя строка «Выбрать / Текущий» (окно «Уровни и функции»).
  let { pick = false, onpick }: { pick?: boolean; onpick?: (l: Level) => void } = $props()

  const groups = featureGroups()

  // Колонка «Свой»: набор; если «Свой» не настраивали — как текущий готовый уровень (первая правка начнёт с него).
  const shown = $derived(prefs.custom ?? fromBase(prefs.level === 'custom' ? 'simple' : prefs.level))
  const set = $derived(rows(shown))

  // Строка статуса: текст и состояние для «Вернуть»; гаснет при следующей правке.
  let status = $state<{ text: string; prev: LevelState } | null>(null)

  function toggle(row: RowId, on: boolean) {
    const e = prefs.toggle(row, on)
    const text = togetherText(row, e.added, e.removed)
    status = text ? { text, prev: e.prev } : null
  }

  function setBase(b: Base) {
    const e = prefs.setBase(b)
    status = { text: resetText(b), prev: e.prev }
  }

  function undo() {
    if (!status) return
    void prefs.apply(status.prev)
    status = null
  }

  // Колонки: три готовых уровня и «Свой».
  const cols: Level[] = [...LEVELS, 'custom']
</script>

<table class="matrix">
  <thead>
    <tr>
      <th class="fn" scope="col">Что доступно</th>
      {#each LEVELS as l (l)}
        <th scope="col" class:cur={prefs.level === l}>{LEVEL_TITLE[l]}</th>
      {/each}
      <th scope="col" class="own" class:cur={prefs.level === 'custom'}>
        <span>Свой</span>
        <label class="base">
          <small>на основе</small>
          <select value={shown.base} onchange={(e) => setBase(normBase(e.currentTarget.value))} aria-label="Основа набора «Свой»">
            {#each LEVELS as l (l)}
              <option value={l}>{LEVEL_TITLE[l]}</option>
            {/each}
          </select>
        </label>
      </th>
    </tr>
  </thead>
  {#each groups as g (g.title)}
    <tbody>
      <tr class="grp"><th colspan="5" scope="colgroup">{g.title}</th></tr>
      {#each g.rows as f (f.id)}
        <tr>
          <td class="fn">
            {f.title}
            <button
              class="help"
              title="Что это и где найти — справочник функций"
              aria-label="Справочник: {f.short}"
              onclick={() => guide.openRow(f.id)}>?</button
            >
          </td>
          {#each LEVELS as l (l)}
            <td class="mark" class:cur={prefs.level === l}>{can(f.id, l) ? '✓' : ''}</td>
          {/each}
          <td class="mark own" class:cur={prefs.level === 'custom'} title={rowTip(f.id)}>
            <input
              type="checkbox"
              checked={set.has(f.id)}
              disabled={locked(f)}
              aria-label="{f.short} — в наборе «Свой»"
              onchange={(e) => toggle(f.id, e.currentTarget.checked)}
            />
          </td>
        </tr>
      {/each}
    </tbody>
  {/each}
  {#if pick}
    <tfoot>
      <tr>
        <td></td>
        {#each cols as l (l)}
          <td class="mark" class:cur={prefs.level === l}>
            {#if prefs.level === l}
              <span class="now">Текущий</span>
            {:else}
              <button onclick={() => onpick?.(l)}>Выбрать</button>
            {/if}
          </td>
        {/each}
      </tr>
    </tfoot>
  {/if}
</table>

<p class="status" aria-live="polite">
  {#if status}
    {status.text} · <button class="link" onclick={undo}>Вернуть</button>
  {/if}
</p>

<style>
  table {
    width: 100%;
    border-collapse: collapse;
  }
  th,
  td {
    padding: 4px 8px;
    border-bottom: 1px solid var(--line);
    text-align: center;
    vertical-align: middle;
  }
  .fn {
    text-align: left;
  }
  /* «?» у строки: справочник по её функциям. Круг мельче строки, чтобы не раздувать таблицу */
  .help {
    width: 18px;
    height: 18px;
    margin-left: 6px;
    padding: 0;
    border-radius: 50%;
    background: transparent;
    border: 1px solid var(--line);
    color: var(--muted);
    font-size: 11px;
    line-height: 1;
    vertical-align: middle;
  }
  .help:hover:not(:disabled) {
    color: var(--accent);
    border-color: var(--accent);
  }
  /* Заголовки на одной линии сверху: у «Свой» шапка выше за счёт списка основы. */
  thead th {
    font-weight: 600;
    white-space: nowrap;
    vertical-align: top;
    padding-top: 6px;
    padding-bottom: 6px;
  }
  .own span {
    display: block;
  }
  .base {
    display: flex;
    flex-direction: column;
    gap: 4px;
    margin-top: 4px;
    font-size: 11px;
    font-weight: 400;
    color: var(--muted);
  }
  .base select {
    width: 100%;
    height: 24px;
    padding: 0 4px;
    font-size: 12px;
  }
  .grp th {
    padding-top: 10px;
    text-align: left;
    font-size: 12px;
    font-weight: 600;
    color: var(--muted);
    text-transform: uppercase;
    letter-spacing: 0.04em;
    border-bottom: 0;
  }
  .mark {
    width: 100px;
    color: var(--accent);
  }
  .mark.own {
    width: 170px;
  }
  input[type='checkbox'] {
    accent-color: var(--accent);
    margin: 0;
  }
  input[type='checkbox']:disabled {
    opacity: 0.5;
  }
  /* Столбец текущего уровня */
  .cur {
    background: rgba(245, 217, 10, 0.08);
  }
  thead th.cur {
    box-shadow: inset 0 -2px 0 var(--accent);
  }
  tfoot td {
    border-bottom: 0;
    padding-top: 10px;
  }
  tfoot .cur {
    background: transparent;
  }
  .now {
    color: var(--muted);
  }
  .status {
    min-height: 20px;
    margin: 8px 0 0;
    color: var(--muted);
  }
  .link {
    height: auto;
    padding: 0;
    background: transparent;
    color: var(--accent);
  }
</style>
