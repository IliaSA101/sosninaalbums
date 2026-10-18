<script lang="ts">
  import { onMount } from 'svelte'
  import { store } from '../../base/core/store.svelte'
  import ColorPicker from '../../base/ui/ColorPicker.svelte'
  import { COVER_LAYOUTS } from '../../editor/cover/cover'
  import MarkupBlock from '../markup/MarkupBlock.svelte'
  import { coverAct } from './coveract.svelte'
  import {
    CUSTOM_ID,
    fieldValue,
    hasTable,
    overLine,
    parseNum,
    presetGroups,
    specWith,
    spineAction,
    spineLine,
    totalLine,
    type SpecField,
  } from './coverpanel'

  // Карточка обложки (спека 032, п. 9–13): типография, размеры, итог, раскладка, плашка корешка, разметка.
  // Все числа — из View.cover; правка любого поля уходит в Go одним шагом, Go переключает типографию на «Свои размеры».
  const cv = $derived(store.cover)
  const groups = $derived(presetGroups(coverAct.presets))
  const custom = $derived(!!cv && !hasTable(cv.spec))
  const over = $derived(cv ? overLine(cv) : '')
  const spine = $derived(spineAction(cv))

  // Не $effect: он зависел от loading внутри loadPresets и при пустом ответе гонял запрос по кругу.
  onMount(() => void coverAct.loadPresets())

  // Enter или уход из поля — в Go; мусор и то же число возвращают прежнее значение; Esc — тоже, без снятия выделения.
  function commit(field: SpecField, el: HTMLInputElement) {
    if (!cv) return
    const cur = fieldValue(cv, field)
    const v = parseNum(el.value)
    if (v === null || v === cur) {
      el.value = String(cur)
      return
    }
    const next = specWith(cv.spec, field, v)
    if (next) void coverAct.setSpec(next)
  }
  function key(field: SpecField, e: KeyboardEvent) {
    const el = e.currentTarget as HTMLInputElement
    if (e.key === 'Enter') el.blur()
    else if (e.key === 'Escape') {
      e.stopPropagation()
      if (cv) el.value = String(fieldValue(cv, field))
      el.blur()
    }
  }
</script>

{#snippet num(field: SpecField, label: string, hint: string)}
  <label class="f" title={hint}>
    {#if label}<span>{label}</span>{/if}
    <input
      type="number"
      min="0"
      step="0.1"
      value={cv ? fieldValue(cv, field) : ''}
      aria-label={hint}
      onchange={(e) => commit(field, e.currentTarget)}
      onkeydown={(e) => key(field, e)}
    />
  </label>
{/snippet}

{#if cv}
  <div class="cover" role="group" aria-label="Обложка">
    <label class="cap" for="cover-preset">Типография</label>
    <select id="cover-preset" data-guide="cover-print" value={cv.preset} onchange={(e) => coverAct.applyPreset(e.currentTarget.value)}>
      {#each groups as g (g.group)}
        {#if g.group}
          <optgroup label={g.group}>
            {#each g.items as p (p.id)}<option value={p.id}>{p.name}</option>{/each}
          </optgroup>
        {:else}
          <!-- без группы — обычные варианты: пустая подпись optgroup рисуется пустой строкой -->
          {#each g.items as p (p.id)}<option value={p.id}>{p.name}</option>{/each}
        {/if}
      {/each}
      {#if !coverAct.presets.some((p) => p.id === cv.preset)}
        <option value={cv.preset}>{cv.preset === CUSTOM_ID ? 'Свои размеры' : cv.preset}</option>
      {/if}
    </select>

    <div class="cap sub">Сторонка Ш × В, мм</div>
    <div class="pair">
      {@render num('w', '', 'Ширина сторонки, мм')}
      <span class="x">×</span>
      {@render num('h', '', 'Высота сторонки, мм')}
    </div>
    <div class="pair">
      {@render num('wrap', 'Заворот', 'Заворот, мм: загибается на картон за линией обреза')}
    </div>
    <div class="pair">
      {@render num('hinge', 'Расстав', 'Расстав, мм: канавка у корешка с каждой стороны')}
    </div>
    {#if custom}
      <div class="cap sub" title="Корешок = a + b × число разворотов; b — толщина одного разворота. Своё число — вручную">
        Корешок = a + b × разворотов
      </div>
      <div class="pair">
        {@render num('a', 'a', 'Корешок = a + b × число разворотов: a — постоянная часть, мм. Своё число — вручную')}
        <span class="x">+</span>
        {@render num(
          'b',
          'b',
          'Корешок = a + b × число разворотов: b — толщина одного разворота, мм (0 — корешок постоянный). Своё число — вручную',
        )}
      </div>
    {:else}
      <div class="hint">Корешок — по таблице типографии</div>
    {/if}

    <div class="total">{totalLine(cv, store.view.settings.dpi)}</div>
    <div class="total">{spineLine(cv)}</div>
    {#if over}<div class="warn">{over}</div>{/if}
    {#if cv.fix}
      <div class="warn">{cv.fix}</div>
      <div class="btns">
        <button onclick={() => coverAct.applyPreset(cv.preset)}>Исправить</button>
      </div>
    {/if}

    <div class="cap sub">Раскладка обложки</div>
    <div class="layouts" role="group" aria-label="Раскладка обложки">
      {#each COVER_LAYOUTS as l (l.kind)}
        <button
          class:on={cv.layout === l.kind}
          aria-pressed={cv.layout === l.kind}
          onclick={() => cv.layout !== l.kind && coverAct.setLayout(l.kind)}>{l.label}</button
        >
      {/each}
    </div>
    <div class="row spine" data-guide="cover-spine">
      <span class="lbl">Корешок</span>
      <button
        class="spinebtn"
        title={spine.kind === 'add' ? 'Текст вдоль корешка по центру, на цветной плашке во весь корешок' : 'Выделить надпись на корешке'}
        onclick={() => coverAct.spineText()}>{spine.label}</button
      >
      <span class="plate">Плашка</span>
      <ColorPicker value={cv.spineBg || '#ffffff'} label="Цвет плашки корешка" onpick={(c) => coverAct.setSpineBg(c)} />
      <button class="reset" disabled={!cv.spineBg} aria-label="Без плашки" title="Без плашки" onclick={() => coverAct.setSpineBg('')}
        >нет</button
      >
    </div>

    <MarkupBlock kind="cover" guide={cv.markup} />

    <button class="ghost del" onclick={() => store.deleteCover()}>Удалить обложку</button>
  </div>
{/if}

<style>
  .cover {
    display: flex;
    flex-direction: column;
    gap: 4px;
    font-size: 12px;
  }
  .cap {
    font-weight: 600;
  }
  .cap.sub {
    margin-top: 8px;
  }
  select {
    width: 100%;
    height: 24px;
    padding: 0 4px;
    font-size: 12px;
  }
  .pair {
    display: flex;
    align-items: center;
    gap: 4px;
  }
  .x {
    color: var(--muted);
  }
  .f {
    flex: 1;
    min-width: 0;
    display: flex;
    align-items: center;
    gap: 4px;
    color: var(--muted);
  }
  .f span {
    flex: none;
    width: 52px;
  }
  .pair:has(.x) .f span {
    width: 10px;
  }
  .f input {
    width: 100%;
    min-width: 0;
    height: 24px;
    padding: 0 4px;
    font-size: 12px;
  }
  .hint {
    color: var(--muted);
  }
  .total {
    margin-top: 2px;
    font-variant-numeric: tabular-nums;
  }
  .total:first-of-type {
    margin-top: 6px;
  }
  .warn {
    color: #e0806f;
  }
  .layouts {
    display: grid;
    /* длинные подписи в две колонки не помещаются — тогда один столбец */
    grid-template-columns: repeat(auto-fit, minmax(170px, 1fr));
    gap: 4px;
  }
  .layouts button {
    height: auto;
    min-height: 26px;
    padding: 3px 4px;
    font-size: 12px;
    white-space: normal;
    line-height: 1.2;
    background: #38383c;
    color: #cfcfd4;
    border-radius: 5px;
  }
  .layouts button.on {
    background: var(--accent);
    color: #1e1e1e;
  }
  .row {
    display: flex;
    align-items: center;
    gap: 6px;
    margin-top: 4px;
  }
  .lbl {
    flex: 1;
    color: var(--muted);
  }
  .row.spine {
    flex-wrap: wrap;
  }
  .row.spine .lbl {
    flex: none;
    width: 52px;
  }
  .spinebtn {
    height: 24px;
    padding: 0 8px;
    font-size: 12px;
  }
  .plate {
    flex: 1;
    min-width: 40px;
    text-align: right;
    color: var(--muted);
  }
  .reset {
    height: 24px;
    padding: 0 6px;
    font-size: 12px;
    background: transparent;
    color: #cfcfd4;
    border-radius: 4px;
  }
  .btns {
    display: flex;
    flex-wrap: wrap; /* узкая колонка: кнопки переносятся, а не сжимаются */
    gap: 6px;
  }
  .btns button {
    flex: 1 0 auto;
    white-space: nowrap;
    height: 24px;
    padding: 0 6px;
    font-size: 12px;
  }
  .del {
    margin-top: 8px;
    height: 24px;
    padding: 0 6px;
    font-size: 12px;
    color: #ff8a8a;
  }
</style>
