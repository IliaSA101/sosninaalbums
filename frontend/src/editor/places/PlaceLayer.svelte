<script lang="ts">
  import type { SpreadView } from '../../base/core/types'
  import { drag } from '../../base/drag/drag.svelte'
  import { placesel } from './placesel.svelte'
  import { moveSelected } from './groupgesture'
  import { additive } from './gesture'
  import { photoui } from '../photo/photoui.svelte'
  import { prefs } from '../../base/prefs/prefs.svelte'
  import { layerIndex } from './layers'
  import { shapeCss } from '../../base/geom/shape'

  // Пустые места сетки (спека 007). Занятые рисует Cell. Пустое место без режима (спека 008):
  // щелчок выделяет (Shift/Ctrl — добавляет), тело переносит место; рамку и ручки рисует SelFrame. Лежат в той же обёртке, что и ячейки:
  // z-index = 10 + z из Go, чтобы бросок и экран совпадали с порядком слоёв при экспорте.
  let { spread, mini = false }: { spread: SpreadView; mini?: boolean } = $props()

  const sw = $derived(spread.w)
  // Выделение — на нажатии: перенос начинается с того же жеста.
  function down(e: PointerEvent, id: string) {
    if (mini || e.button !== 0) return
    e.stopPropagation()
    // Пустое место «Ручной» на низком уровне: видно, но не выделяется (вопрос 5).
    // «Включить» в тосте после включения выделяет это место, как обычный щелчок.
    if (prefs.blocked('place', () => placesel.pick(id, false))) return
    if (additive(e)) return placesel.pick(id, true)
    if (!placesel.refs.includes(id)) placesel.pick(id, false)
    moveSelected(e, (e.currentTarget as HTMLElement).closest<HTMLElement>('.spread')!, () => placesel.select([id]))
  }
  // Щелчок по месту не снимает выделение (его снимает щелчок по фону разворота).
  const click = (e: MouseEvent) => e.stopPropagation()

  const empties = $derived((spread.places ?? []).filter((p) => !p.photo))
</script>

{#each empties as p (p.id)}
  <div
    class="place"
    class:mini
    class:spine={p.spine}
    class:over={!mini && drag.over === `place:${p.id}`}
    style:left="{(p.x / sw) * 100}%"
    style:top="{(p.y / spread.h) * 100}%"
    style:width="{(p.w / sw) * 100}%"
    style:height="{(p.h / spread.h) * 100}%"
    style:z-index={layerIndex(p.z)}
    style:border-radius={shapeCss(p.shape, placesel.liveRadius([p.id], p.radius ?? 0, p.w, p.h), p.w, p.h)}
    style:transform={p.rot ? `rotate(${p.rot}deg)` : undefined}
    data-drop={mini ? undefined : 'place'}
    data-spread={spread.id}
    data-place={p.id}
    title={p.spine && !mini ? 'Место пересекает корешок, а альбом не раскрывается плоско' : undefined}
    role="presentation"
    onpointerdown={(e) => down(e, p.id)}
    onclick={mini ? undefined : click}
    oncontextmenu={mini ? undefined : (e) => photoui.open(e, p.id)}
  >
    {#if !mini}<span class="hint">+ фото</span>{/if}
  </div>
{/each}

<style>
  .place {
    position: absolute;
    box-sizing: border-box;
    display: grid;
    place-items: center;
    overflow: hidden;
    cursor: move;
    border: 1px dashed #7aa7ff;
    background: rgba(122, 167, 255, 0.08);
    color: #7aa7ff;
    font-size: 13px;
    white-space: nowrap;
    transition:
      background 0.15s,
      border-color 0.15s;
  }
  .place.mini {
    cursor: inherit;
    border: none;
    background: #d4d4d8;
  }
  .place.spine {
    border-color: #e0803a;
  }
  .place.mini.spine {
    background: #d4d4d8;
    box-shadow: inset 0 0 0 1px #e0803a;
  }
  .place.over {
    border: 2px dashed #7aa7ff;
    background: rgba(59, 130, 246, 0.25);
  }
  .hint {
    pointer-events: none;
    user-select: none;
  }
</style>
