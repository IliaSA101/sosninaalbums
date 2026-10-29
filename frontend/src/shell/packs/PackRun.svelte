<script lang="ts">
  import type { LibNet } from './packnet.svelte'
  import { progressPercent, progressText } from './packnet'

  // Полоса хода скачивания (067 Ф3, 137): «Скачиваем «…» — 12 из 40 МБ», полоса, «Отменить». Одна на окно «Наборы» и галереи
  // «Фоны», «Стикеры», «Рамочки», «Развороты», «Макеты альбома» (144): ход общий, `packsUi.net`. Нет хода — ничего не рисует.
  let { net }: { net: LibNet } = $props()

  const working = $derived(net.updating || !!net.progress)
  const pct = $derived(progressPercent(net.progress))
</script>

{#if working}
  <div class="progress">
    <div class="row">
      <span role="status">{net.cancelling ? 'Отменяем…' : progressText(net.progress)}</span>
      <span class="grow"></span>
      <button class="ghost" disabled={net.cancelling} onclick={() => net.cancel()}>Отменить</button>
    </div>
    <progress max="100" value={pct ?? undefined} aria-label="Ход обновления библиотеки"></progress>
  </div>
{/if}

<style>
  .progress {
    flex: none;
    margin: 0 0 10px;
    padding: 8px 10px;
    border: 1px solid var(--line);
    border-radius: 6px;
    background: var(--panel2);
  }
  .row {
    display: flex;
    align-items: center;
    gap: 10px;
    margin-bottom: 6px;
    font-variant-numeric: tabular-nums;
  }
  .grow {
    flex: 1;
  }
  progress {
    width: 100%;
    height: 6px;
    accent-color: var(--accent);
  }
</style>
