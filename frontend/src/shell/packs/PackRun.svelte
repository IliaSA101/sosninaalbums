<script lang="ts">
  import ProgressRun from '../../base/ui/ProgressRun.svelte'
  import type { LibNet } from './packnet.svelte'
  import { progressPercent, progressText } from './packnet'

  // Полоса хода скачивания (067 Ф3, 137): «Скачиваем «…» — 12 из 40 МБ», полоса, «Отменить». Одна на окно «Наборы» и галереи
  // «Фоны», «Стикеры», «Рамочки», «Развороты», «Макеты альбома» (144): ход общий, `packsUi.net`. Нет хода — ничего не рисует.
  // Вид — общий `base/ui/ProgressRun` (его же берёт окно ролика F1).
  let { net }: { net: LibNet } = $props()

  const working = $derived(net.updating || !!net.progress)
</script>

{#if working}
  <div class="gap">
    <ProgressRun
      text={progressText(net.progress)}
      percent={progressPercent(net.progress)}
      label="Ход обновления библиотеки"
      cancelling={net.cancelling}
      oncancel={() => net.cancel()}
    />
  </div>
{/if}

<style>
  /* Нижний отступ и «не сжиматься» в колонке окна — как у прежней полосы */
  .gap {
    flex: none;
    margin: 0 0 10px;
  }
</style>
