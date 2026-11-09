<script lang="ts">
  import { tipped } from './tip.svelte'

  // Красный «!» в правом верхнем углу плитки разворота: есть кадры или фон с малым разрешением.
  // Лежит в отступе плитки (content-visibility обрежет всё за её краем). Родитель — position: relative.
  let { count, min }: { count: number; min: number } = $props()
  const text = $derived(`Мало разрешения для печати: ${count} фото (нужно от ${min} dpi)`)
</script>

{#if count > 0}
  <span class="low" role="img" aria-label="Мало разрешения" use:tipped={{ text }}>!</span>
{/if}

<style>
  .low {
    position: absolute;
    top: 1px;
    right: 1px;
    width: 14px;
    height: 14px;
    border-radius: 50%;
    background: #d9382c;
    color: #fff;
    font-size: 10px;
    font-weight: 800;
    line-height: 14px;
    text-align: center;
    z-index: 4;
  }
</style>
