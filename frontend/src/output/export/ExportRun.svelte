<script lang="ts">
  import { jobs } from '../../base/core/jobs.svelte'
  import { runLine, runPercent } from './exportrun'

  // Ход выгрузки в окне «Экспорт» (073): вид и слово кнопки — как у сбора в папку (CollectDialog).
  // Окно можно закрыть: выгрузка идёт, шапка и меню открывают его снова.
  const pct = $derived(runPercent(jobs.exporting))
  const stopping = $derived(jobs.exportStopping)
</script>

<div class="exrun">
  <progress max="100" value={pct ?? undefined} aria-label="Ход выгрузки"></progress>
  <div class="line" role="status">{runLine(jobs.exporting)}</div>
  <p class="hint">Уже выгруженные файлы остаются. Окно можно закрыть — выгрузка продолжится.</p>
  <div class="actions">
    <button data-autofocus disabled={stopping} onclick={() => jobs.stopExport()}>{stopping ? 'Останавливаем…' : 'Остановить'}</button>
  </div>
</div>

<style>
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
  .hint {
    color: var(--muted);
    font-size: 12px;
    line-height: 1.45;
    margin: 0 0 8px;
  }
  .actions {
    display: flex;
    justify-content: flex-end;
    margin-top: 14px;
  }
</style>
