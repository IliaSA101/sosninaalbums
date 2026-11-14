<script lang="ts">
  import Icon from './Icon.svelte'

  // Заглушка «файла нет»: серый блок со значком «фото с минусом». Общий вид места на развороте (Cell),
  // плитки ленты (Thumb) и крупного просмотра (PhotoPreview). Занимает весь родитель (position: relative).
  // label — короткая подпись под значком; видна, только если родитель — контейнер шире 90 px (мелкая плитка: один значок).
  let { size = 28, tip, label = '' }: { size?: number; tip: string; label?: string } = $props()
</script>

<div class="miss" role="img" aria-label={tip} title={tip}>
  <Icon name="photo-minus" {size} stroke={1.8} />
  {#if label}<span class="lbl">{label}</span>{/if}
</div>

<style>
  .miss {
    position: absolute;
    inset: 0;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 4px;
    background: #6a6a6e;
    color: #cfcfd2;
  }
  .lbl {
    display: none;
    font-size: 11px;
    line-height: 1.2;
    max-width: 100%;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  @container (min-width: 90px) {
    .lbl {
      display: block;
    }
  }
</style>
