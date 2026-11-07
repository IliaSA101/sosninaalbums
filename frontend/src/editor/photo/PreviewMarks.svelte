<script lang="ts">
  import { stars } from '../../panels/marks/thumbmark'
  import type { Mark } from '../../base/core/types'
  import Icon from '../../base/ui/Icon.svelte'

  // Отметки показанного фото в подписи окна крупного просмотра (128): звёзды, флажок, цвет метки.
  let { mark }: { mark: Mark } = $props()
</script>

<span class="mk">
  {#if mark.rating > 0}<span class="stars">{stars(mark.rating)}</span>{/if}
  {#if mark.flag === 1}<Icon name="flag" size={14} />{:else if mark.flag === -1}<Icon name="flag-off" size={14} />{/if}
  {#if mark.label !== ''}<i class="dot {mark.label}"></i>{/if}
</span>

<style>
  .mk {
    flex: 1;
    display: inline-flex;
    align-items: center;
    gap: 8px;
    color: #fff;
  }
  .stars {
    letter-spacing: 1px;
  }
  .dot {
    width: 11px;
    height: 11px;
    border-radius: 50%;
    background: var(--lbl-other);
  }
  .dot.red {
    background: var(--lbl-red);
  }
  .dot.yellow {
    background: var(--lbl-yellow);
  }
  .dot.green {
    background: var(--lbl-green);
  }
  .dot.blue {
    background: var(--lbl-blue);
  }
  .dot.purple {
    background: var(--lbl-purple);
  }
</style>
