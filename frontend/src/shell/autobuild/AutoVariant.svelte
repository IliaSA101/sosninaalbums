<script lang="ts">
  import Icon from '../../base/ui/Icon.svelte'
  import type { PhotosState } from './autosimilar.svelte'

  // «Вариант ‹ 2 ›» у кнопок окна (078-8): стрелки меняют номер, «Что соберётся» показывает этот вариант.
  // Устройство — как «Раскладка ‹ 2 / 7 ›» над разворотом (SpreadBar .variants).
  let { ph, off = false }: { ph: PhotosState; off?: boolean } = $props()
</script>

<div class="variant" role="group" aria-label="Вариант сборки" title="Другой вариант сборки тех же фото">
  <button
    type="button"
    class="icon"
    disabled={off || ph.variant <= 1}
    onclick={() => ph.setVariant(ph.variant - 1)}
    title="Предыдущий вариант"
    aria-label="Предыдущий вариант"><Icon name="caret-left" size={12} /></button
  >
  <span class="label" role="status">Вариант {ph.variant}</span>
  <button
    type="button"
    class="icon"
    disabled={off}
    onclick={() => ph.setVariant(ph.variant + 1)}
    title="Следующий вариант"
    aria-label="Следующий вариант"><Icon name="caret-right" size={12} /></button
  >
</div>

<style>
  .variant {
    display: flex;
    align-items: center;
    gap: 6px;
    margin-right: auto; /* кнопки «Отмена» и «Собрать» уходят вправо */
    color: var(--muted);
    font-size: 13px;
  }
  .label {
    min-width: 72px;
    text-align: center;
    font-variant-numeric: tabular-nums;
  }
</style>
