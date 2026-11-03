<script lang="ts">
  import { tick } from 'svelte'
  import Icon from '../../base/ui/Icon.svelte'
  import { printEdit as ui } from './printedit.svelte'
  import { NO_FORMATS, fieldErr, pageCell, pageOk, rowBad, sideCell, sideOk, spineNote } from './printwords'

  // Таблица форматов окна «Своя типография»: подпись · файл разворота px = страница мм · файл обложки px = сторонка мм.
  // Миллиметры и ошибки строк — из ответа Go (ui.check.formats[i]); здесь только показ.
  let rows = $state<HTMLElement>()
  async function add() {
    ui.addRow()
    await tick()
    rows?.querySelector<HTMLInputElement>('[data-row]:last-of-type input')?.focus()
  }
</script>

<p class="sec">Форматы</p>
<div class="tbl" bind:this={rows}>
  <div class="gr head">
    <span>Подпись</span><span>Файл разворота, px</span><span>= страница, мм</span><span>Файл обложки, px</span><span>= сторонка, мм</span
    ><span></span>
  </div>
  {#each ui.e.formats as f, i (i)}
    {@const r = ui.check?.formats?.[i]}
    {@const err = r?.err ?? ''}
    <div class="gr" data-row={i}>
      <input type="text" class:bad={rowBad(r, 'name')} aria-label="Подпись формата" spellcheck="false" bind:value={f.name} />
      <span class="pair">
        <input type="number" class:bad={rowBad(r, 'px')} aria-label="Ширина разворота, px" min="0" bind:value={f.pxW} />
        <span class="x">×</span>
        <input type="number" class:bad={rowBad(r, 'px')} aria-label="Высота разворота, px" min="0" bind:value={f.pxH} />
      </span>
      <span class="mm" class:none={!pageOk(r)}>{pageCell(r)}</span>
      <span class="pair">
        <input type="number" class:bad={rowBad(r, 'cover')} aria-label="Ширина обложки, px" min="0" bind:value={f.coverW} />
        <span class="x">×</span>
        <input type="number" class:bad={rowBad(r, 'cover')} aria-label="Высота обложки, px" min="0" bind:value={f.coverH} />
      </span>
      <span class="mm" class:none={!sideOk(r)}>{sideCell(r, f.coverW, f.coverH)}</span>
      <button class="del" type="button" title="Убрать формат" aria-label="Убрать формат" onclick={() => ui.delRow(i)}>
        <Icon name="x" size={12} />
      </button>
    </div>
    {#if err}<div class="gerr"><p class="errt" role="alert">{err}</p></div>{/if}
  {/each}
  {#if ui.e.formats.length === 0}<p class="hint empty">{NO_FORMATS}</p>{/if}
</div>
{#if fieldErr(ui.check, 'formats')}<p class="errt top" role="alert">{fieldErr(ui.check, 'formats')}</p>{/if}
<div class="under">
  <button class="ghost" type="button" disabled={!ui.canAdd} onclick={add}>+ Добавить формат</button>
  <span class="sp"></span>
  <!-- px обложки зависят от числа разворотов через корешок: при каком числе они введены (0 — как в альбоме) -->
  <label for="pe-spreads"
    >Обложка — при
    <input id="pe-spreads" type="number" class:bad={!!fieldErr(ui.check, 'spreads')} min="1" max="100" bind:value={ui.e.spreads} />
    разворотах{spineNote(ui.check?.spine ?? 0)}</label
  >
  {#if fieldErr(ui.check, 'spreads')}<p class="errt" role="alert">{fieldErr(ui.check, 'spreads')}</p>{/if}
</div>

<style>
  .sec {
    font-weight: 600;
    margin: 0 0 6px;
  }
  .tbl {
    border: 1px solid var(--line);
    border-radius: 6px;
    padding: 6px 8px;
    margin-bottom: 8px;
    max-height: 232px;
    overflow: auto;
  }
  .gr {
    display: grid;
    grid-template-columns: minmax(90px, 1.1fr) 150px minmax(112px, 1fr) 150px minmax(112px, 1fr) 26px;
    gap: 4px 8px;
    align-items: center;
    padding: 3px 0;
  }
  .gr:not(.head) + .gr,
  .gr + .gerr + .gr {
    border-top: 1px solid var(--line);
  }
  .gr.head {
    color: var(--muted);
    font-size: 12px;
    padding-top: 0;
  }
  .gr input {
    height: 26px;
    font-size: 13px;
    padding: 0 6px;
    min-width: 0;
  }
  .gr input[type='text'] {
    width: 100%;
    font: inherit;
    font-size: 13px;
    background: #3a3a3e;
    color: var(--text);
    border: 1px solid var(--line);
    border-radius: 6px;
    user-select: text;
  }
  .gr input[type='number'] {
    width: 100%;
    appearance: textfield;
  }
  input.bad {
    border-color: #e5645a;
  }
  .pair {
    display: flex;
    align-items: center;
    gap: 4px;
  }
  .x {
    color: var(--muted);
  }
  .mm {
    font-variant-numeric: tabular-nums;
    font-size: 13px;
  }
  .mm.none {
    color: var(--muted);
  }
  .del {
    width: 26px;
    height: 26px;
    padding: 0;
    display: grid;
    place-items: center;
    background: transparent;
    color: var(--muted);
  }
  .gerr {
    padding: 0 0 4px;
  }
  .errt {
    margin: 0;
    font-size: 12px;
    line-height: 1.35;
    color: #e5645a;
    overflow-wrap: anywhere;
  }
  .errt.top {
    margin: -4px 0 6px;
  }
  .hint {
    margin: 0;
    font-size: 12px;
    color: var(--muted);
  }
  .hint.empty {
    padding: 6px 0;
  }
  .under {
    display: flex;
    align-items: center;
    gap: 12px;
    flex-wrap: wrap;
    margin-bottom: 12px;
  }
  .under .sp {
    flex: 1;
  }
  .under label {
    display: flex;
    align-items: center;
    gap: 6px;
    color: var(--muted);
    font-size: 12px;
  }
  .under input {
    width: 60px;
    height: 26px;
    font-size: 13px;
    padding: 0 6px;
  }
</style>
