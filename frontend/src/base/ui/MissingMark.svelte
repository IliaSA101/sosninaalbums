<script lang="ts">
  import Icon from './Icon.svelte'
  import { tipped } from './tip.svelte'

  // Метка «файл фото не найден» в плитке разворота (лента): светлый кружок со значком фото, левее красного «!»
  // малого разрешения (LowDpiBadge), чтобы не путать с ним. Лежит в отступе плитки (content-visibility
  // обрежет всё за её краем). Родитель — position: relative.
  let { count }: { count: number } = $props()
  const text = $derived(count === 1 ? 'Файл фото не найден' : `Файлы не найдены: ${count} фото`)
</script>

{#if count > 0}
  <span class="miss" role="img" aria-label="Фото не найдено" use:tipped={{ text }}><Icon name="photo-minus" size={10} stroke={2.2} /></span>
{/if}

<style>
  .miss {
    position: absolute;
    top: 1px;
    right: 17px;
    width: 14px;
    height: 14px;
    border-radius: 50%;
    background: var(--text);
    color: #1e1e1e;
    display: grid;
    place-items: center;
    z-index: 4;
  }
</style>
