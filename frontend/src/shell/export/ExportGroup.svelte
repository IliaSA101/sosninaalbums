<script lang="ts">
  import type { Snippet } from 'svelte'

  // Раздел окна «Экспорт»: подпись слева, варианты справа (вынесено из ExportDialog, 075-11).
  // off — раздел серый (режим печати или «только обложка»); role/label — для групп радиокнопок.
  let {
    cap,
    off = false,
    role,
    label,
    children,
  }: {
    cap: string
    off?: boolean
    role?: 'radiogroup'
    label?: string
    children: Snippet
  } = $props()
</script>

<div class="group" class:off {role} aria-label={label}>
  <span class="cap">{cap}</span>
  <div class="opts">{@render children()}</div>
</div>

<style>
  /* Окно шире обычного: строки «Развороты» и «Вид» помещаются в одну строку */
  :global(.box.wide):has(.opts) {
    width: 560px;
  }
  /* Подпись строки — левая колонка 64 px, варианты — правая; при переносе держатся левого края вариантов. */
  .group {
    display: grid;
    grid-template-columns: 64px 1fr;
    align-items: center;
    gap: 0 14px;
    margin-bottom: 12px;
  }
  .opts {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 6px 14px;
    min-width: 0;
  }
  .cap {
    color: var(--muted);
    font-size: 12px;
  }
  .group.off {
    opacity: 0.5;
  }
</style>
