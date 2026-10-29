<script lang="ts">
  import { store } from '../../base/core/store.svelte'
  import CardGroup from '../../editor/inspector/CardGroup.svelte'
  import { sizeSum } from '../../editor/inspector/groupsum'
  import MarkupBlock from '../markup/MarkupBlock.svelte'
  import { printList } from '../../shell/printprof/printlist.svelte'
  import { coverAct } from './coveract.svelte'
  import CoverPx from './CoverPx.svelte'
  import {
    CUSTOM_ID,
    fieldValue,
    hasTable,
    parseNum,
    presetGroups,
    presetName,
    specWith,
    spineLine,
    totalLine,
    type SpecField,
  } from './coverpanel'

  // Группа «Размеры для типографии» (спека 032, п. 9–13; 124 U6): типография, размеры сторонки, итог, файл обложки
  // в px (123) и разметка. Все числа — из View.cover; правка любого поля уходит в Go одним шагом, Go переключает
  // типографию на «Свои размеры». Предупреждения («Типография не найдена», «Не по шаблону») — над группами.
  const cv = $derived(store.cover)
  const groups = $derived(presetGroups(coverAct.presets))
  const custom = $derived(!!cv && !hasTable(cv.spec))
  const summary = $derived(cv ? sizeSum(presetName(coverAct.presets, cv.preset), spineLine(cv)) : '')

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
  <CardGroup id="cover.size" title="Размеры для типографии" {summary}>
    <div class="size" role="group" aria-label="Размеры для типографии">
      <label class="cap" for="cover-preset">Типография</label>
      <select id="cover-preset" data-guide="cover-print" value={cv.preset} onchange={(e) => coverAct.applyPreset(e.currentTarget.value)}>
        {#each groups as g (g.group)}
          {#if g.group}
            <optgroup label={printList.label(g.group)}>
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
      <CoverPx />

      <MarkupBlock kind="cover" guide={cv.markup} />
    </div>
  </CardGroup>
{/if}

<style>
  .size {
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
</style>
