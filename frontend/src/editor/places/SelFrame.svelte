<script lang="ts">
  import { primaryPress } from '../../base/core/platform'
  import type { Handle, SpreadView } from '../../base/core/types'
  import { placesel } from './placesel.svelte'
  import { startMove, startResize, startRotate, zeroAngle } from './placegesture'
  import { aabbOf, angleOf, resizeCursor, rotHandleAt, rotHandleIn } from '../../base/geom/rotbox'
  import { photoui } from '../photo/photoui.svelte'
  import { hidesRotate, tiltmode } from '../tilt/tiltmode.svelte'
  import { store } from '../../base/core/store.svelte'
  import { prefs } from '../../base/prefs/prefs.svelte'
  import MiniPanel from './MiniPanel.svelte'
  import RotateHandle from '../../base/ui/RotateHandle.svelte'

  // Рамка выделенного на развороте (спека 008, modeless.md): 8 ручек у образца (первого), по краю
  // рамки полоса 10 px — перенос места; тело под рамкой остаётся фото (обмен, ✥, ползунок — в Cell).
  // Повёрнутое место — рамка крутится вокруг центра вместе с ручками и ⟳. Лежит над слоем текста. Жесты — places/placegesture.ts; предпросмотр — placesel.live (сама рамка
  // едет за курсором, фото встаёт на место после ответа Go).
  let { spread, width }: { spread: SpreadView; width: number } = $props()

  const sw = $derived(spread.w)
  const ph = $derived(spread.h)
  const ppm = $derived(width / sw) // px на мм
  const handles: Handle[] = ['nw', 'n', 'ne', 'e', 'se', 's', 'sw', 'w']
  const corners: Handle[] = ['nw', 'ne', 'se', 'sw']
  const hx = (h: Handle) => (h.includes('w') ? 0 : h.includes('e') ? 100 : 50)
  const hy = (h: Handle) => (h.includes('n') ? 0 : h.includes('s') ? 100 : 50)
  // Меньше 48 px на экране — только угловые ручки (середины сторон не помещаются).
  const SMALL = 48

  const live = $derived(placesel.live?.spread === spread.id ? placesel.live.boxes : null)
  const items = $derived(
    placesel.selected.map((b, k) => ({
      ref: b.ref,
      id: b.id,
      photo: b.photo,
      fit: b.fit,
      box: live?.get(b.id) ?? { x: b.x, y: b.y, w: b.w, h: b.h, rot: b.rot },
      sample: k === 0,
    })),
  )
  // Группа (2+ объекта, в том числе тексты): рамка, ручки и мини-панель — у GroupFrame, здесь обводки членов.
  const group = $derived(placesel.isGroup)
  // Края (перенос места) и ручки размера — «Ручная раскладка»; ниже рамка только обводит выделенное.
  const manual = $derived(prefs.can('manual'))
  const rotatable = $derived(manual && prefs.can('rotate'))
  const ROT_REACH = 30 // px от верхней стороны до ⟳
  const ROT_MID = 24 // px от стороны до центра кружка ⟳ (RotateHandle)
  // Мини-панель одного места — пока жеста нет.
  const panelBox = $derived.by(() => {
    if (live || items.length !== 1 || group) return null
    return aabbOf(items[0].box) // повёрнутое — по описанному прямоугольнику
  })

  // Центр ⟳ одиночного места (px разворота): мини-панель обходит его, иначе закрывает ручку.
  // В режиме наклона ⟳ спрятан (на его месте дуга) — обходить нечего.
  const noRot = (photo: string | undefined) => hidesRotate(tiltmode.on, photo, store.selectedCell)
  const rotAt = $derived.by(() => {
    if (!panelBox || !rotatable || noRot(items[0].photo)) return undefined
    const b = items[0].box
    const p = rotHandleAt(b, (rotHandleIn(b, ROT_REACH / ppm, sw, ph) ? -ROT_MID : ROT_MID) / ppm)
    return { x: p.x * ppm, y: p.y * ppm }
  })

  let root: HTMLDivElement

  const pc = (v: number, of: number) => `${(v / of) * 100}%`
  const down = (e: PointerEvent, ref: string) => {
    if (!primaryPress(e)) return
    e.stopPropagation()
    startMove(e, root, ref)
  }
  const resize = (e: PointerEvent, h: Handle) => {
    if (!primaryPress(e)) return
    e.stopPropagation()
    startResize(e, root, h)
  }
  const rotate = (e: PointerEvent) => startRotate(e, root)
  // Клик по краю — не клик по фону: выделение остаётся.
  const keep = (e: MouseEvent) => e.stopPropagation()
</script>

<div bind:this={root} class="selframe" role="presentation">
  {#each items as it (it.id)}
    {@const small = Math.min(it.box.w, it.box.h) * ppm < SMALL}
    <div
      class="frame"
      class:sample={it.sample && !group}
      class:live={!!live}
      style:left={pc(it.box.x, sw)}
      style:top={pc(it.box.y, ph)}
      style:width={pc(it.box.w, sw)}
      style:height={pc(it.box.h, ph)}
      style:transform={angleOf(it.box) ? `rotate(${angleOf(it.box)}deg)` : undefined}
    >
      {#each group || !manual ? [] : ['t', 'b', 'l', 'r'] as side (side)}
        <span
          class="edge {side}"
          role="presentation"
          onpointerdown={(e) => down(e, it.ref)}
          onclick={keep}
          oncontextmenu={(e) => photoui.open(e, it.ref)}
        ></span>
      {/each}
      {#if it.sample && !group && manual}
        {#each small ? corners : handles as h (h)}
          <span
            class="h {h}"
            style:left="{hx(h)}%"
            style:top="{hy(h)}%"
            style:cursor={resizeCursor(h, angleOf(it.box))}
            role="presentation"
            onpointerdown={(e) => resize(e, h)}
            onclick={keep}
          ></span>
        {/each}
      {/if}
      {#if it.sample && !group && rotatable && !noRot(it.photo)}
        <RotateHandle accept={primaryPress} rotIn={rotHandleIn(it.box, ROT_REACH / ppm, sw, ph)} onrotate={rotate} onzero={zeroAngle} />
      {/if}
    </div>
  {/each}

  {#if panelBox}
    <MiniPanel {spread} box={panelBox} {width} height={(width * ph) / sw} {ppm} avoid={rotAt} />
  {/if}

  {#if placesel.guides.x !== null}
    <div class="guide gv" style:left={pc(placesel.guides.x, sw)}></div>
  {/if}
  {#if placesel.guides.y !== null}
    <div class="guide gh" style:top={pc(placesel.guides.y, ph)}></div>
  {/if}
</div>

<style>
  .selframe {
    position: absolute;
    inset: 0;
    z-index: 7; /* над всем контентом (.content, 3), притемнением (5) и безопасной зоной (6) */
    pointer-events: none;
  }
  .frame {
    position: absolute;
    box-sizing: border-box;
    outline: 2px solid var(--accent);
    outline-offset: -1px;
  }
  .frame.sample {
    outline-width: 3px;
  }
  .frame.live {
    background: rgba(245, 217, 10, 0.18);
  }
  /* Полоса по краю внутри рамки: перенос места. У совсем мелкого места не больше трети стороны. */
  .edge {
    position: absolute;
    pointer-events: auto;
    cursor: move;
    touch-action: none;
  }
  .edge.t,
  .edge.b {
    left: 0;
    right: 0;
    height: min(10px, 33%);
  }
  .edge.t {
    top: 0;
  }
  .edge.b {
    bottom: 0;
  }
  .edge.l,
  .edge.r {
    top: 0;
    bottom: 0;
    width: min(10px, 33%);
  }
  .edge.l {
    left: 0;
  }
  .edge.r {
    right: 0;
  }
  /* Ручка: видимая 8 px, зона попадания 14 px (тачпад); курсор — по углу места (resizeCursor). */
  .h {
    position: absolute;
    width: 14px;
    height: 14px;
    transform: translate(-50%, -50%);
    pointer-events: auto;
    touch-action: none;
  }
  .h::before {
    content: '';
    position: absolute;
    left: 3px;
    top: 3px;
    width: 8px;
    height: 8px;
    box-sizing: border-box;
    background: #fff;
    border: 1.5px solid #1e1e1e;
    border-radius: 2px;
  }
  .guide {
    position: absolute;
    pointer-events: none;
    background: #4aa3ff;
  }
  .guide.gv {
    top: 0;
    bottom: 0;
    width: 1px;
  }
  .guide.gh {
    left: 0;
    right: 0;
    height: 1px;
  }
</style>
