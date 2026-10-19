<script lang="ts">
  import { store } from '../../base/core/store.svelte'
  import type { BorderView } from '../../base/core/types'
  import Icon from '../../base/ui/Icon.svelte'
  import { borderSrc } from './border'
  import { borderState } from './borderstate'

  // Накладка рамочки поверх фото или пустого места (079): прозрачный PNG от Go (/media), рамка места целиком —
  // родитель задаёт ей размер, поворот и скругление (border-radius: inherit). Рамочка не прозрачнеет вместе с фото
  // (opacity у .clip ячейки, не здесь). Нет адреса (мягкий край — только маска фото) — рисовать нечего.
  // U5: вид новее программы (known = false) и картинка без файла — пунктир по рамке места, как у стикера; у пропавшей
  // ещё значок «фото нет» в углу. Подсказку ставит родитель (title на ячейке), потому что мышь накладка не ловит:
  // щелчок, выделение и drag идут сквозь рамочку в место.
  let { border, px, mini = false }: { border: BorderView; px: number; mini?: boolean } = $props()
  const state = $derived(borderState(border, store.missing))
</script>

{#if state !== 'ok'}
  <div class="gone" class:mini data-border-gone={state}>
    {#if state === 'lost' && !mini}<span class="miss" role="img" aria-label="Файл рамочки не найден"
        ><Icon name="photo-minus" size={16} stroke={2} /></span
      >{/if}
  </div>
{:else if border.url}
  <img class="bo" data-border={border.kind || border.photo} src={borderSrc(border.url, px)} alt="" draggable="false" />
{/if}

<style>
  .bo {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
    max-width: none;
    display: block;
    pointer-events: none;
    user-select: none;
  }
  /* Пунктир светлый с тёмной каймой: виден и на тёмном, и на светлом фото. */
  .gone {
    position: absolute;
    inset: 0;
    box-sizing: border-box;
    border: 1.5px dashed rgba(255, 255, 255, 0.95);
    border-radius: inherit;
    box-shadow:
      0 0 0 1px rgba(0, 0, 0, 0.45),
      inset 0 0 0 1px rgba(0, 0, 0, 0.45);
    pointer-events: none;
    user-select: none;
  }
  .gone.mini {
    border-width: 1px;
  }
  .miss {
    position: absolute;
    right: 4px;
    top: 4px;
    display: grid;
    place-items: center;
    padding: 2px;
    border-radius: 4px;
    background: rgba(0, 0, 0, 0.55);
    color: #fff;
  }
</style>
