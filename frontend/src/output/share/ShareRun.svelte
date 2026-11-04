<script lang="ts">
  import { share } from './share.svelte'
  import { runLine, runPercent } from './share'
  import { RUN_HINT, RUN_LABEL, STOPPING_LABEL, STOP_LABEL } from './sharetext'

  // Ход выгрузки (146): вид и слово кнопки — как у ExportRun (073). Окно можно закрыть — выгрузка идёт, меню открывает его снова.
  const pct = $derived(runPercent(share.progress))
</script>

<div class="run">
  <progress max="100" value={pct ?? undefined} aria-label={RUN_LABEL}></progress>
  <div class="line" role="status">{runLine(share.progress)}</div>
  <p class="hint">{RUN_HINT}</p>
  <div class="actions">
    <button data-autofocus disabled={share.stopping} onclick={() => share.stop()}>{share.stopping ? STOPPING_LABEL : STOP_LABEL}</button>
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
