<script lang="ts">
  import { jobs } from '../../base/core/jobs.svelte'
  import Modal from '../../base/modal/Modal.svelte'
  import { openLine, openPercent } from './openprogress'

  // Окно «Открываем альбом» (073): ход прогрева раскладок большого альбома. Вид — как ход сбора (CollectDialog),
  // но без кнопок: прогрев не отменяется. Гаснет само — на (total, total) и на смене альбома (jobs.endOpening).
  const shown = $derived(jobs.openShown && jobs.opening !== null)
  const pct = $derived(openPercent(jobs.opening))
</script>

{#if shown}
  <!-- onclose пуст: Esc и клик по фону окно не закрывают -->
  <Modal title="Открываем альбом" onclose={() => {}} width={420}>
    <div class="opening">
      <progress max="100" value={pct ?? undefined} aria-label="Ход подготовки раскладок"></progress>
      <div class="line" role="status">{openLine(jobs.opening)}</div>
    </div>
  </Modal>
{/if}

<style>
  /* Закрыть нельзя — крестик Modal не показываем (как во время сбора в папку) */
  :global(.box:has(.opening) .x) {
    visibility: hidden;
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
</style>
