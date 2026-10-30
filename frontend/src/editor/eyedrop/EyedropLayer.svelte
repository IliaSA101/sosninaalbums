<script lang="ts">
  import { portal } from '../../base/ui/portal'
  import { primaryPress } from '../../base/core/platform'
  import type { SpreadView } from '../../base/core/types'
  import { Api, store } from '../../base/core/store.svelte'
  import { eyedrop } from '../../base/color/eyedrop.svelte'
  import { modalStack } from '../../base/modal/modalstack.svelte'
  import { errText } from '../../base/words/errtext'
  import { canvas } from '../canvas/canvas.svelte'
  import { LOUPE, loupeAt, pointQueue, sheetPoint } from './eyedropat'

  // Слой режима «Взять цвет с фото» (109, design §7) над листом окна цвета: курсор-перекрестье, лупа с цветом под
  // курсором, щелчок берёт цвет. Цвет считает Go по кадру листа (PickColorAt): как в печати, а не как на экране.
  let { spread }: { spread: SpreadView } = $props()

  let root = $state<HTMLElement>()
  // Лупа: позиция в px окна; hex — последний ответ Go (null — ещё не ответил, «…»).
  let at = $state<{ left: number; top: number } | null>(null)
  let hex = $state<string | null>(null)
  let taking = false // щелчок ушёл в Go, второй не нужен

  // Один запрос в пути, следующий берёт последнюю точку. Ошибка лупы (точка у самой кромки) — «…», режим идёт.
  const queue = pointQueue(
    (p) => Api.PickColorAt(spread.id, p.x, p.y),
    (_p, h) => (hex = h),
    () => (hex = null),
  )
  $effect(() => queue.stop) // слой ушёл — ответы в пути отбрасываются

  function move(e: PointerEvent) {
    const r = root?.getBoundingClientRect()
    const p = r && sheetPoint(e.clientX, e.clientY, r, spread.w)
    if (!p) {
      at = null
      return
    }
    at = loupeAt(e.clientX, e.clientY, window.innerWidth, window.innerHeight)
    queue.push(p)
  }

  async function down(e: PointerEvent) {
    // Пробел-рука и не-основная кнопка идут дальше: панорама холста работает, как в других режимах.
    if (!primaryPress(e) || canvas.space || !root || taking) return
    e.stopPropagation()
    e.preventDefault()
    const p = sheetPoint(e.clientX, e.clientY, root.getBoundingClientRect(), spread.w)
    if (!p) return
    const owner = eyedrop.owner
    taking = true
    queue.stop()
    try {
      const h = await Api.PickColorAt(spread.id, p.x, p.y)
      if (eyedrop.owner === owner) eyedrop.finish(h) // за время ответа режим могли снять — тогда цвет не нужен
    } catch (err) {
      if (eyedrop.owner === owner) eyedrop.cancel()
      store.notify(errText(err), 'error')
    }
  }

  // Esc — до остальных обработчиков (фаза захвата): выход из режима, окно цвета остаётся. Если окно уже отработало
  // этот Esc само (режим снят) — мы не вмешиваемся; иначе гасим событие, чтобы окно не закрылось тем же нажатием.
  function key(e: KeyboardEvent) {
    if (!eyedrop.on || e.key !== 'Escape' || modalStack.any()) return
    e.stopImmediatePropagation()
    e.preventDefault()
    eyedrop.cancel()
  }
</script>

<svelte:window onkeydowncapture={key} />

<!-- click, dblclick и меню глушим: иначе щелчок после выбора дошёл бы до поля (снял бы выделение) и до «вписать» -->
<div
  class="layer"
  bind:this={root}
  data-bd-ui
  data-eyedrop
  data-text-ui
  role="presentation"
  onpointerdown={down}
  onpointermove={move}
  onpointerleave={() => (at = null)}
  onclick={(e) => e.stopPropagation()}
  ondblclick={(e) => e.stopPropagation()}
  oncontextmenu={(e) => {
    e.stopPropagation()
    e.preventDefault()
  }}
></div>

{#if at}
  <div
    class="loupe"
    use:portal
    style:left="{at.left}px"
    style:top="{at.top}px"
    style:width="{LOUPE}px"
    style:height="{LOUPE}px"
    style:background={hex ?? 'transparent'}
  >
    <span>{hex ?? '…'}</span>
  </div>
{/if}

<style>
  /* Выше рамки фона (40), слоя «Выпрямить» (45) и рамок выделения */
  .layer {
    position: absolute;
    inset: 0;
    z-index: 46;
    cursor: crosshair;
    touch-action: none;
  }
  /* Круг с цветом: белая рамка и тонкий чёрный контур — виден и на белом, и на чёрном (макет .loupe) */
  .loupe {
    position: fixed;
    z-index: 98;
    box-sizing: border-box;
    border-radius: 50%;
    border: 3px solid #fff;
    box-shadow:
      0 0 0 1px #000,
      0 4px 12px rgba(0, 0, 0, 0.5);
    pointer-events: none;
  }
  .loupe span {
    position: absolute;
    top: 50px;
    left: 50%;
    transform: translateX(-50%);
    padding: 1px 5px;
    border-radius: 3px;
    background: #111;
    color: #fff;
    font:
      11px Consolas,
      monospace;
    white-space: nowrap;
  }
</style>
