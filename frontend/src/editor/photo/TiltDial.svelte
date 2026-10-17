<script lang="ts">
  import type { Frame, FrameView, SpreadView } from '../../base/core/types'
  import { Api, store } from '../../base/core/store.svelte'
  import Icon from '../../base/ui/Icon.svelte'
  import { canvas } from '../canvas/canvas.svelte'
  import { markDone } from '../places/placegesture'
  import { framelive } from './framelive.svelte'
  import { canvasTilt, liveTilt } from './tiltlive'
  import { DialControl, dialTicks, fmtTilt, sameTilt, TILT_MAX } from './tiltdial'
  import { straighten } from './straighten.svelte'
  import { tiltmode } from './tiltmode.svelte'

  // Шкала наклона на выделенном фото (спека 059): вместо ползунка масштаба. Линейка едет под неподвижной стрелкой,
  // 1 px = 0,1°. Рамка стоит, фото на холсте поворачивается сразу (liveTilt — зеркало Go TiltCrop); правка
  // SetFrameAngle уходит на отпускании: одна протяжка — один шаг отмены. Решения — в tiltdial.ts.
  let {
    id,
    spread,
    fv,
    frame,
    shaped,
  }: {
    id: string
    spread: SpreadView
    fv: FrameView // место с фото: без него шкалы нет (CellOverlay)
    frame: Frame // кадр из Go
    shaped: boolean // круг и скругление: нижний угол срезан
  } = $props()

  let live = $state<number | null>(null) // угол, пока тянут или жмут стрелки
  const t = $derived(live ?? fv.tilt ?? 0)
  let rw = $state(172) // ширина линейки, px (замер)
  const ticks = $derived(dialTicks(t, rw))

  // Протяжка и сессия отмены ведёт ctl (tiltdial.ts); base — кадр начала, основа предпросмотра.
  const ctl = new DialControl({
    session: canvasTilt,
    key: () => id,
    frame: () => frame,
    tilt: () => fv.tilt ?? 0,
    rev: () => store.view.rev,
    send: (angle, session) => store.run(Api.SetFrameAngle(spread.id, id, angle, session)),
  })
  let base: Frame | null = null
  let lastFrame: Frame | null = null

  function preview(angle: number, from: Frame) {
    const f = liveTilt(spread, id, from, angle)
    if (!f) return null
    framelive.set(id, f, true, false, angle) // угол — для строки «Наклон» в карточке
    return f
  }

  function down(e: PointerEvent) {
    if (e.button !== 0 || ctl.dragging) return
    e.preventDefault() // фокус не берём: стрелки холста после протяжки по-прежнему двигают место
    e.stopPropagation()
    ;(e.currentTarget as HTMLElement).setPointerCapture(e.pointerId)
    canvas.busy = true
    base = ctl.down(e.clientX, e.clientY)
    lastFrame = null
    tiltmode.dialing = true
    window.addEventListener('keydown', esc, true) // фаза захвата: раньше выхода из режима и «снять выделение»
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

  // Esc во время протяжки: откат предпросмотра без правки, режим шкалы остаётся.
  function esc(e: KeyboardEvent) {
    if (e.key !== 'Escape' || !ctl.dragging) return
    e.stopPropagation()
    e.preventDefault()
    ctl.cancel()
    framelive.clear()
    live = null
  }

  // Двойной щелчок по линейке — 0° одной правкой (если уже 0 — холостого вызова нет).
  async function reset() {
    if (sameTilt(fv.tilt ?? 0, 0)) return
    markDone()
    await ctl.reset()
  }

  // ←/→ на шкале в фокусе (Tab): 0,1°, с Shift 1°. Серия до ухода фокуса — один шаг отмены.
  async function key(e: KeyboardEvent) {
    if (e.target !== e.currentTarget) return // кнопка-уровень со своими клавишами
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

  // Щелчок мышью по шкале не ставит ей фокус: иначе ←/→ молча менялись бы наклон вместо сдвига места (решение 19).
  // Фокус берётся только Tab. Кнопка-уровень внутри по-прежнему кликается.
  function shield(e: PointerEvent) {
    e.preventDefault()
    e.stopPropagation()
  }

  function focusout(e: FocusEvent) {
    if (!(e.currentTarget as HTMLElement).contains(e.relatedTarget as Node | null)) ctl.blur()
  }

  // Шкала исчезла (смена выделения, выход из режима): незаконченная протяжка не должна оставить busy и сетку.
  $effect(() => () => {
    if (ctl.dispose()) {
      stop()
      framelive.clear()
    }
  })
</script>

<!-- Клики и двойной щелчок не доходят до холста: иначе щелчок по шкале снял бы выделение -->
<div
  class="dial"
  class:shaped
  role="slider"
  tabindex="0"
  aria-label="Наклон фото в рамке"
  aria-valuemin={-TILT_MAX}
  aria-valuemax={TILT_MAX}
  aria-valuenow={t}
  aria-valuetext={fmtTilt(t)}
  title="Наклон фото в рамке; двойной щелчок — 0°"
  data-guide="tilt-mini"
  onpointerdown={shield}
  onmousedown={(e) => e.preventDefault()}
  onclick={(e) => e.stopPropagation()}
  ondblclick={(e) => e.stopPropagation()}
  onkeydown={key}
  onfocusout={focusout}
>
  <button
    class="lvl"
    data-guide="straighten"
    aria-label="Выпрямить по линии"
    title="Выпрямить по линии: проведите линию вдоль горизонта или вертикали"
    onclick={() => straighten.startPhoto(id)}><Icon name="level" size={16} /></button
  >
  <span class="sep"></span>
  <div
    class="ruler"
    role="presentation"
    bind:clientWidth={rw}
    onpointerdown={down}
    onpointermove={move}
    onpointerup={up}
    onpointercancel={abort}
    onlostpointercapture={abort}
    ondblclick={reset}
  >
    <svg width={rw} height="36" viewBox="0 0 {rw} 36" aria-hidden="true">
      {#each ticks as k (k.x)}
        <line x1={k.x} x2={k.x} y1="28" y2={28 - (k.big ? 10 : 5)} class:zero={k.label === '0'} stroke-opacity={k.big ? 0.95 : 0.55} />
        {#if k.label}<text x={k.x} y="12" text-anchor="middle">{k.label}</text>{/if}
      {/each}
    </svg>
    <span class="ptr"></span>
  </div>
  <span class="sep"></span>
  <span class="val">{fmtTilt(t)}</span>
</div>

<style>
  /* Внизу ячейки, как ползунок масштаба: подложка, порог и скрытие при переносе те же; шире него — 140…260 px */
  .dial {
    position: absolute;
    left: 50%;
    bottom: 12px;
    width: clamp(140px, 80%, 260px);
    height: 36px;
    transform: translateX(-50%);
    display: flex;
    align-items: center;
    box-sizing: border-box;
    background: rgba(20, 20, 22, 0.84);
    border-radius: 8px;
    color: #cfcfd4;
    z-index: 4;
    pointer-events: auto; /* обёртка CellOverlay мышь не ловит, а свойство наследуется */
    outline: none;
  }
  .dial:focus-visible {
    box-shadow: 0 0 0 2px var(--accent);
  }
  /* У круга и скругления нижний угол срезан: шкала выше */
  .dial.shaped {
    bottom: calc(12px + 8%);
  }
  :global(body.dragging) .dial {
    display: none;
  }
  .lvl {
    width: 24px;
    height: 24px;
    margin: 0 4px;
    padding: 0;
    flex: none;
    display: grid;
    place-items: center;
    border-radius: 4px;
    background: #38383c;
    color: #cfcfd4;
  }
  .lvl:hover {
    background: #48484d;
    color: #fff;
  }
  .sep {
    width: 1px;
    height: 18px;
    flex: none;
    background: var(--line);
  }
  .ruler {
    position: relative;
    flex: 1;
    min-width: 0;
    height: 100%;
    overflow: hidden;
    cursor: ew-resize;
    touch-action: none; /* тачпад и перо: протяжка, а не прокрутка */
  }
  .ruler svg {
    position: absolute;
    top: 0;
    left: 0;
  }
  line {
    stroke: #cfcfd4;
    stroke-width: 1;
  }
  line.zero {
    stroke: var(--accent);
    stroke-width: 2;
  }
  text {
    font-size: 9px;
    fill: var(--muted);
  }
  /* Неподвижная стрелка в середине линейки */
  .ptr {
    position: absolute;
    left: 50%;
    bottom: 0;
    margin-left: -5px;
    border: 5px solid transparent;
    border-bottom: 6px solid var(--accent);
    pointer-events: none;
  }
  .val {
    width: 52px;
    flex: none;
    text-align: center;
    font-size: 12px;
    font-weight: 600;
    font-variant-numeric: tabular-nums;
    color: #fff;
  }
</style>
