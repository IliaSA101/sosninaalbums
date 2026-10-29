<script lang="ts">
  import { FILE_DPI, exportPx, parsePx } from '../details/presets'
  import { Api, store } from '../../base/core/store.svelte'
  import { coverPxNote } from './coverpanel'

  // Строка «Файл обложки, px: Ш × В» (123, design §1.3): ввод уходит в Go одним шагом отмены (SetCoverPx, 300 dpi),
  // сторонку подбирает Go. Поля — как «Сторонка Ш × В»: Enter или уход из поля, мусор и то же число возвращают прежнее.
  const cv = $derived(store.cover)
  const pw = $derived(cv ? exportPx(cv.sheet.w, FILE_DPI) : 0)
  const ph = $derived(cv ? exportPx(cv.sheet.h, FILE_DPI) : 0)
  let wEl = $state<HTMLInputElement>()
  let hEl = $state<HTMLInputElement>()

  function back() {
    if (wEl) wEl.value = String(pw)
    if (hEl) hEl.value = String(ph)
  }
  async function commit() {
    const w = parsePx(wEl?.value ?? '')
    const h = parsePx(hEl?.value ?? '')
    if (w === null || h === null || (w === pw && h === ph)) return back()
    // Ошибку Go показывает тостом (store.run); поля при отказе возвращаются к листу обложки.
    if (!(await store.run(Api.SetCoverPx(w, h)))) back()
  }
  function key(e: KeyboardEvent) {
    const el = e.currentTarget as HTMLInputElement
    if (e.key === 'Enter') el.blur()
    else if (e.key === 'Escape') {
      e.stopPropagation()
      back()
      el.blur()
    }
  }
</script>

{#if cv}
  <div class="cap" data-guide="cover-px">Файл обложки, px: Ш × В</div>
  <div class="pair">
    <input
      type="number"
      min="1"
      step="1"
      value={pw}
      aria-label="Ширина файла обложки, px"
      bind:this={wEl}
      onchange={commit}
      onkeydown={key}
    />
    <span class="x">×</span>
    <input
      type="number"
      min="1"
      step="1"
      value={ph}
      aria-label="Высота файла обложки, px"
      bind:this={hEl}
      onchange={commit}
      onkeydown={key}
    />
  </div>
  <div class="hint">{coverPxNote(cv)}</div>
{/if}

<style>
  .cap {
    margin-top: 8px;
    font-weight: 600;
  }
  .pair {
    display: flex;
    align-items: center;
    gap: 4px;
  }
  .x,
  .hint {
    color: var(--muted);
  }
  input {
    flex: 1;
    min-width: 0;
    height: 24px;
    padding: 0 4px;
    font-size: 12px;
  }
</style>
