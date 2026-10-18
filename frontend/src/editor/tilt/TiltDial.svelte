<script lang="ts">
  import type { Frame, FrameView, SpreadView } from '../../base/core/types'
  import { Api, store } from '../../base/core/store.svelte'
  import Icon from '../../base/ui/Icon.svelte'
  import { canvas } from '../canvas/canvas.svelte'
  import { markDone } from '../places/placegesture'
  import type { ArcRoom } from '../places/minipanel'
  import { framelive } from '../photo/framelive.svelte'
  import { canvasTilt, liveTilt } from './tiltlive'
  import { ARC_PHI, arcAngle, arcGeom, axisDelta, DialControl, fmtTilt, knobAt, KNOB, sameTilt, TILT_ARC } from './tiltdial'
  import { straighten } from './straighten.svelte'
  import { tiltmode } from './tiltmode.svelte'

  // Дуга наклона сверху на выделенном фото (спека 063): вместо ползунка масштаба. Полоса и кружок — как у ползунка
  // масштаба (8 px #3b3b3b, ⌀ 18 акцент), заливка от вершины (0°) до кружка. Вправо — плюс. Рамка стоит, фото на
  // холсте поворачивается сразу (liveTilt — зеркало Go TiltCrop); правка SetFrameAngle уходит на отпускании:
  // одна протяжка — один шаг отмены. Геометрия и перевод px → угол — в tiltdial.ts; место дуги даёт мини-панель (room).
  let {
    id,
    spread,
    fv,
    frame,
    room,
    shaped,
  }: {
    id: string
    spread: SpreadView
    fv: FrameView // место с фото: без него дуги нет (CellOverlay)
    frame: Frame // кадр из Go
    room: ArcRoom // где дуга: внутри фото или над рамкой
    shaped: boolean // круг и скругление: нижний угол срезан, дуга ниже
  } = $props()

  let live = $state<number | null>(null) // угол, пока тянут или жмут стрелки
  const t = $derived(live ?? fv.tilt ?? 0)
  let rw = $state(0) // размер места в осях места, px (замер)
  let rh = $state(0)
  const g = $derived(arcGeom(room, rw, rh, shaped))
  const phi = (ARC_PHI * Math.PI) / 180
  const at = (a: number) => ({ x: g.ax + g.R * Math.sin(a), y: g.ay + g.R * (1 - Math.cos(a)) })
  const left = $derived(at(-phi))
  const right = $derived(at(phi))
  const top = $derived(at(0))
  const knob = $derived(knobAt(t, g))
  const arcD = $derived(`M${left.x} ${left.y}A${g.R} ${g.R} 0 0 1 ${right.x} ${right.y}`)
  const fillD = $derived(
    Math.abs(knob.x - top.x) < 0.01
      ? ''
      : knob.x > top.x
        ? `M${top.x} ${top.y}A${g.R} ${g.R} 0 0 1 ${knob.x} ${knob.y}`
        : `M${knob.x} ${knob.y}A${g.R} ${g.R} 0 0 1 ${top.x} ${top.y}`,
  )

  // Протяжка и сессия отмены ведёт ctl (tiltdial.ts); base — кадр начала, основа предпросмотра.
  const ctl = new DialControl({
    session: canvasTilt,
    key: () => id,
    frame: () => frame,
    tilt: () => fv.tilt ?? 0,
    rev: () => store.view.rev,
    // Кружок держится под курсором по x оси места и стоит на дуге
    toAngle: (t0, dpx, m) => arcAngle(knobAt(t0, g).x + dpx, g, m),
    send: (angle, session) => store.run(Api.SetFrameAngle(spread.id, id, angle, session)),
  })
  let base: Frame | null = null
  let lastFrame: Frame | null = null
  let knobEl: SVGCircleElement | undefined

  function preview(angle: number, from: Frame) {
    const f = liveTilt(spread, id, from, angle)
    if (!f) return null
    framelive.set(id, f, true, false, angle) // угол — для строки «Наклон» в карточке
    return f
  }

  function begin(e: PointerEvent, jump?: number) {
    if (e.button !== 0 || ctl.dragging) return
    e.preventDefault() // фокус не берём: стрелки холста после протяжки по-прежнему двигают место
    e.stopPropagation()
    ;(e.currentTarget as Element).setPointerCapture(e.pointerId)
    canvas.busy = true
    base = ctl.down(e.clientX, e.clientY, jump, { shift: e.shiftKey, alt: e.altKey })
    lastFrame = null
    tiltmode.dialing = true
    window.addEventListener('keydown', esc, true) // фаза захвата: раньше выхода из режима и «снять выделение»
    const a = ctl.startAngle
    if (a !== null && base) {
      live = a
      lastFrame = preview(a, base)
    }
  }

  const downKnob = (e: PointerEvent) => begin(e)
  // Нажатие на полосу: кружок встаёт под курсор (сдвиг от центра кружка по оси места), дальше протяжка
  function downTrack(e: PointerEvent) {
    const r = knobEl?.getBoundingClientRect()
    if (!r) return
    begin(e, axisDelta(e.clientX - (r.left + r.width / 2), e.clientY - (r.top + r.height / 2), fv.cellRot ?? 0))
  }

  function move(e: PointerEvent) {
    if (!base) return
    const a = ctl.move(e.clientX, e.clientY, fv.cellRot ?? 0, { shift: e.shiftKey, alt: e.altKey })
    if (a === null) return
    live = a
    lastFrame = preview(a, base) ?? lastFrame
  }

  function stop() {
    window.removeEventListener('keydown', esc, true)
    canvas.busy = false
    tiltmode.dialing = false
    base = null
  }

  async function up() {
    if (!ctl.dragging) return
    const a = ctl.release()
    stop()
    if (a === null) {
      framelive.clear()
      live = null
      return
    }
    markDone() // click после протяжки не должен снимать выделение
    if (lastFrame) framelive.set(id, lastFrame, false, false, a) // кадр остаётся до ответа Go, сетка гаснет
    await ctl.commit(a)
    // За время ответа Go могла начаться новая протяжка: её предпросмотр не стираем.
    if (!ctl.dragging) {
      framelive.clear()
      live = null
    }
  }

  // pointercancel и потеря захвата: протяжка сорвана системой — как Esc, без записи угла. После обычного
  // отпускания протяжки уже нет, и повторный вызов ничего не делает.
  function abort() {
    if (!ctl.dragging) return
    ctl.cancel()
    void up()
  }

  // Esc во время протяжки: откат предпросмотра без правки, режим наклона остаётся.
  function esc(e: KeyboardEvent) {
    if (e.key !== 'Escape' || !ctl.dragging) return
    e.stopPropagation()
    e.preventDefault()
    ctl.cancel()
    framelive.clear()
    live = null
  }

  // Двойной щелчок по кружку — 0° одной правкой (если уже 0 — холостого вызова нет).
  async function reset() {
    if (sameTilt(fv.tilt ?? 0, 0)) return
    markDone()
    await ctl.reset()
  }

  // ←/→ на кружке в фокусе (Tab): 0,1°, с Shift 1°; на дуге до ±10°. Серия до ухода фокуса — один шаг отмены.
  async function key(e: KeyboardEvent) {
    const step = ctl.key(t, e.key, e.shiftKey)
    if (!step) return
    e.preventDefault()
    e.stopPropagation() // не двигать место стрелками холста
    if (!step.base) return
    live = step.angle
    preview(step.angle, step.base)
    await step.commit()
    if (live === step.angle) {
      live = null
      framelive.clear()
    }
  }

  // Щелчок мышью по кнопке и дуге не ставит им фокус и не доходит до холста (решение 19): фокус берётся только Tab.
  function shield(e: Event) {
    e.preventDefault()
    e.stopPropagation()
  }
  const quiet = (e: Event) => e.stopPropagation()

  // Дуга исчезла (смена выделения, выход из режима): незаконченная протяжка не должна оставить busy и сетку.
  $effect(() => () => {
    if (ctl.dispose()) {
      stop()
      framelive.clear()
    }
  })
</script>

<!-- Клики и двойной щелчок не доходят до холста: иначе щелчок по дуге снял бы выделение -->
<div class="arc" bind:clientWidth={rw} bind:clientHeight={rh}>
  <svg width="1" height="1" onpointermove={move} onpointerup={up} onpointercancel={abort} onlostpointercapture={abort} role="presentation">
    <path d={arcD} class="bar" />
    {#if fillD}<path d={fillD} class="fill" />{/if}
    <path d={arcD} class="hit" role="presentation" onpointerdown={downTrack} onmousedown={shield} onclick={quiet} ondblclick={quiet} />
    <g
      class="knob"
      class:grab={tiltmode.dialing}
      role="slider"
      tabindex="0"
      aria-label="Наклон фото в рамке"
      aria-valuemin={-TILT_ARC}
      aria-valuemax={TILT_ARC}
      aria-valuenow={Math.min(TILT_ARC, Math.max(-TILT_ARC, t))}
      aria-valuetext={fmtTilt(t)}
      data-guide="tilt-mini"
      onpointerdown={downKnob}
      onmousedown={shield}
      onclick={quiet}
      ondblclick={(e) => {
        e.stopPropagation()
        void reset()
      }}
      onkeydown={key}
      onfocusout={() => ctl.blur()}
    >
      <title>Наклон фото в рамке; двойной щелчок — 0°</title>
      <circle cx={knob.x} cy={knob.y} r="12" class="zone" />
      <circle bind:this={knobEl} cx={knob.x} cy={knob.y} r={KNOB} class="dot" />
    </g>
  </svg>
  <button
    class="lvl"
    data-guide="straighten"
    aria-label="Выпрямить по линии"
    title="Выпрямить по линии: проведите линию вдоль горизонта или вертикали"
    style:left="{left.x - 34}px"
    style:top="{left.y - 12}px"
    onpointerdown={shield}
    onclick={(e) => {
      e.stopPropagation()
      straighten.startPhoto(id)
    }}><Icon name="level" size={16} /></button
  >
  <span class="val" style:left="{right.x + 10}px" style:top="{right.y - 11}px">{fmtTilt(t)}</span>
  {#if tiltmode.dialing}
    <span class="tag" style:left="{knob.x}px" style:top="{knob.y + 14}px">{fmtTilt(t)}</span>
  {/if}
</div>

<style>
  /* Оболочка на всё место (оси места, поворот вместе с ним); мышь ловят только дуга, кружок и боковые элементы */
  .arc {
    position: absolute;
    inset: 0;
    z-index: 4;
    pointer-events: none;
  }
  :global(body.dragging) .arc {
    display: none;
  }
  svg {
    position: absolute;
    left: 0;
    top: 0;
    overflow: visible;
    touch-action: none; /* тачпад и перо: протяжка, а не прокрутка */
  }
  path {
    fill: none;
    stroke-linecap: round;
  }
  /* Как нативный ползунок масштаба: полоса 8 px #3b3b3b, заливка акцентом до кружка */
  .bar {
    stroke: #3b3b3b;
    stroke-width: 8;
  }
  .fill {
    stroke: var(--accent);
    stroke-width: 8;
  }
  .hit {
    stroke: transparent;
    stroke-width: 22;
    pointer-events: stroke;
    cursor: pointer;
  }
  .knob {
    pointer-events: all;
    cursor: grab;
    outline: none;
  }
  .knob.grab {
    cursor: grabbing;
  }
  .zone {
    fill: transparent;
  }
  .dot {
    fill: var(--accent);
  }
  .knob:focus-visible .dot {
    stroke: #fff;
    stroke-width: 2;
  }
  .lvl {
    position: absolute;
    width: 24px;
    height: 24px;
    padding: 0;
    display: grid;
    place-items: center;
    border-radius: 4px;
    background: #38383c;
    color: #cfcfd4;
    pointer-events: auto; /* оболочка мышь не ловит, а свойство наследуется */
  }
  .lvl:hover {
    background: #48484d;
    color: #fff;
  }
  .val,
  .tag {
    position: absolute;
    display: grid;
    place-items: center;
    border-radius: 4px;
    background: rgba(20, 20, 22, 0.84);
    color: #fff;
    font-weight: 600;
    font-variant-numeric: tabular-nums;
    white-space: nowrap;
    pointer-events: none;
  }
  .val {
    height: 22px;
    padding: 0 6px;
    font-size: 12px;
  }
  .tag {
    height: 18px;
    padding: 0 5px;
    font-size: 11px;
    transform: translateX(-50%);
  }
</style>
