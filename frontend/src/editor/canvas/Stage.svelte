<script lang="ts">
  import SpreadSlots from '../spread/SpreadSlots.svelte'
  import LayoutBar from '../../panels/layouts/LayoutBar.svelte'
  import BackdropBar from '../backdrop/BackdropBar.svelte'
  import ZoomBadge from '../zoom/ZoomBadge.svelte'
  import EditRail from '../inspector/EditRail.svelte'
  import Inspector from '../inspector/Inspector.svelte'
  import Marquee from './Marquee.svelte'
  import Rulers from './Rulers.svelte'
  import { store } from '../../base/core/store.svelte'
  import { drag } from '../../base/drag/drag.svelte'
  import { modes } from '../../base/prefs/view.svelte'
  import { canvas } from './canvas.svelte'
  import { row } from '../spread/spreadrow.svelte'
  import { GAP, pitch, slotLeft } from '../spread/spreadrow'
  import { bgmode } from '../backdrop/bgmode.svelte'
  import { prefs } from '../../base/prefs/prefs.svelte'
  import { emptyField } from './field'
  import { fieldClick } from '../places/placegesture'
  import { isZoomWheel, placement, wheelFactor, wheelPan } from '../zoom/zoom'
  import { BAR, railCols, sideReserve } from '../inspector/panels'

  let w = $state(0)
  let h = $state(0)
  let el = $state<HTMLElement>()
  let marq = $state<ReturnType<typeof Marquee>>()

  const s = $derived(store.view.settings)
  const sw = $derived(s.pageW * 2)
  const aspect = $derived(sw / s.pageH)
  // В крупном просмотре отступы уже — разворот крупнее. side — минимум видимого соседа при узком запасе.
  const side = $derived(modes.big ? 56 : 110)
  const edge = $derived(modes.big ? 8 : 16) // минимальный отступ ряда от краёв окна
  // Полоса над холстом (BAR = 32) — LayoutBar.
  // Запас по бокам под ленту и карточку (inspector/panels.ts): постоянный, от режима правки не зависит.
  const reserve = $derived(sideReserve(side, GAP, modes.details || !prefs.can('panels'), railCols(h - BAR)))
  // Ширина вписанного разворота (100 %). Без нижнего предела: он выталкивал разворот за Stage,
  // когда высоту отнимала галерея.
  const width = $derived(Math.max(0, Math.min(w - 2 * reserve - 2 * edge, (h - BAR - 32) * aspect)))
  const total = $derived(store.view.spreads.length)

  // Окно холста — Stage под полосой; при 100 % центр окна = центр прежнего ряда, вход в зум не дёргает.
  $effect.pre(() => {
    canvas.vp = { w, h: Math.max(0, h - BAR), k0: sw > 0 ? width / sw : 0, sw, ph: s.pageH, pad: reserve }
  })
  // Режим холста: соседей нет, полоса прижата к верху во всю ширину.
  const zoomed = $derived(canvas.zoomed && !!store.spread)
  const fitPlace = $derived(placement({ z: 1, cx: sw / 2, cy: s.pageH / 2 }, canvas.vp))
  const place = $derived(zoomed ? placement(canvas.cur, canvas.vp) : fitPlace)
  // Разворот получает ширину в px с зумом: перевод px ↔ мм во всех жестах идёт через неё.
  const spreadW = $derived(zoomed ? width * canvas.cur.z : width)

  // Ряд разворотов (спека 012): шаг, число слотов и зум — в ряд; сам он следует за store.current.
  const P = $derived(pitch(fitPlace.width))
  $effect.pre(() => row.geo(P, total, zoomed))
  $effect.pre(() => {
    void store.navSeq // go() на тот же разворот (со слота «новый» назад) тоже возвращает ряд
    row.follow(store.current)
  })
  // Левый край текущего в окне: полоса и линейки едут за ним; в движении полоса гаснет.
  const curLeft = $derived(zoomed ? place.left : slotLeft(store.current, row.pos, fitPlace.left, P))
  const away = $derived(row.moving && Math.abs(row.pos - store.current) > 0.01)

  // Курсор над разворотом или полосой — показываем значки раскладок.
  let hovSpread = $state(false)
  let hovBar = $state(false)
  const hovered = $derived(hovSpread || hovBar)

  // Масштаб копится до кадра: тачпад шлёт десятки событий за кадр.
  let factor = 1
  let fx = 0
  let fy = 0
  let raf = 0
  function zoomTo(f: number, x: number, y: number) {
    factor *= f
    fx = x
    fy = y
    if (raf) return
    raf = requestAnimationFrame(() => {
      raf = 0
      row.finish() // зум не начинается посреди движения ряда
      canvas.zoomAt(factor, fx, fy)
      factor = 1
    })
  }

  // Посреди жеста (перенос, кадр, текст, места) масштаб не меняется: иначе px на мм поплыли бы.
  const busy = () => canvas.busy || canvas.pressed || drag.active

  function wheel(e: WheelEvent) {
    // Над полосой и панелями — их прокрутка; Ctrl+колесо гасим всегда, иначе масштабируется страница.
    if ((e.target as HTMLElement).closest('[data-chrome]')) {
      if (isZoomWheel(e)) e.preventDefault()
      return
    }
    e.preventDefault()
    // Ctrl+колесо и щипок — зум к курсору. Без них: в зуме — сдвиг листа (колесо вниз-вверх, Shift — вбок, тачпад — оба),
    // при 100 % — ряд разворотов; во время переноса фото ряд едет (цель пересчитывается), посреди жеста правки ждёт.
    if (isZoomWheel(e)) {
      // На слоте «новый» разворота под курсором нет: зум не начинаем (вид прыгнул бы)
      if (!store.spread || busy() || row.atNew || !prefs.can('zoom')) return
      const r = el!.getBoundingClientRect()
      zoomTo(wheelFactor(e.deltaY, e.deltaMode), e.clientX - r.left, e.clientY - r.top - BAR)
    } else if (zoomed) {
      if (!store.spread || busy() || !prefs.can('zoom')) return
      const d = wheelPan(e)
      canvas.panBy(d.dx, d.dy)
    } else if (drag.active || !(canvas.busy || canvas.pressed)) row.wheel(e)
  }
  // Свой слушатель с passive: false — иначе preventDefault у колеса не сработает.
  $effect(() => {
    const node = el
    if (!node) return
    node.addEventListener('wheel', wheel, { passive: false })
    return () => {
      node.removeEventListener('wheel', wheel)
      if (raf) cancelAnimationFrame(raf)
      raf = 0
      factor = 1
    }
  })

  // Панорама: зажатое колёсико или Пробел + левая кнопка (при 100 % они тащат ряд). Фаза захвата — перенос
  // фото и жесты текста под курсором не начинаются.
  let panId: number | null = null
  let lx = 0
  let ly = 0
  let swallowClick = false
  function swallow() {
    swallowClick = true
    setTimeout(() => (swallowClick = false), 0)
  }
  // Ряд отпущен: если его сдвигали, щелчок после этого не должен перейти на соседа.
  function ended(moved: boolean) {
    canvas.endPan()
    if (moved) swallow()
  }
  function down(e: PointerEvent) {
    const t = e.target as HTMLElement
    if (t.closest('[data-chrome]')) return // полоса и панели — не холст
    if (e.button === 1 || (e.button === 0 && canvas.space)) {
      e.preventDefault() // средняя кнопка: иначе автопрокрутка Chromium
      e.stopPropagation()
      canvas.startPan()
      if (!zoomed) {
        // 100 %: тащим ряд разворотов (в режиме фона и без разворотов — ничего)
        if (!row.grab(e, ended)) canvas.endPan()
        return
      }
      el!.setPointerCapture(e.pointerId)
      panId = e.pointerId
      lx = e.clientX
      ly = e.clientY
      return
    }
    if (e.button === 0) {
      // Сосед: протяжка двигает ряд, щелчок без сдвига уйдёт в его слой
      if (t.closest('[data-peer]') && row.grab(e, ended)) return
      canvas.pressed = true
      // Протяжка по пустому полю и серому полю вокруг разворота — выделение рамкой; в режиме фона её нет.
      if (!bgmode.cur && !row.moving && prefs.can('groupSelect')) marq?.start(e)
    }
  }
  function move(e: PointerEvent) {
    if (e.pointerId !== panId) return
    canvas.panBy(e.clientX - lx, e.clientY - ly)
    lx = e.clientX
    ly = e.clientY
  }
  function up(e: PointerEvent) {
    if (e.pointerId !== panId) return
    panId = null
    canvas.endPan()
    swallow() // щелчок после панорамы не должен выделять фото под курсором
  }
  function click(e: MouseEvent) {
    if (!swallowClick) return
    e.stopPropagation()
    e.preventDefault()
  }

  // Щелчок по серому полю вокруг разворота снимает выделение, как щелчок по полю внутри (Spread.bgClick).
  // Внутри .spread работает сам Spread; в режиме фона поле обрабатывает BackdropEditor.outside.
  function fieldTap(e: MouseEvent) {
    const t = e.target as Element
    if (!bgmode.cur && !t.closest('.spread') && emptyField(t)) fieldClick(e)
  }

  // Двойной щелчок по фону (серое поле или пустая часть страницы) — вписать.
  function dbl(e: MouseEvent) {
    if (emptyField(e.target as Element) && store.spread) canvas.fit()
  }
</script>

<svelte:window
  onpointerupcapture={() => (canvas.pressed = false)}
  onpointercancelcapture={() => (canvas.pressed = false)}
  onblur={() => {
    canvas.reset()
    row.finish()
  }}
/>

<div
  class="stage"
  class:grab={canvas.space}
  class:grabbing={canvas.panning}
  bind:this={el}
  bind:clientWidth={w}
  bind:clientHeight={h}
  role="presentation"
  onpointerdowncapture={down}
  onpointermove={move}
  onpointerup={up}
  onpointercancel={up}
  onlostpointercapture={up}
  onmousedown={(e) => e.button === 1 && e.preventDefault()}
  onclickcapture={click}
  onclick={fieldTap}
  ondblclick={dbl}
>
  {#if w > 0}
    <!-- Полоса над холстом: постоянные px от краёв Stage, при зуме стоит; над разворотом едет за текущим -->
    <div class="topbar" role="presentation" data-chrome onmouseenter={() => (hovBar = true)} onmouseleave={() => (hovBar = false)}>
      {#if store.spread}
        <!-- Рамка: при 100 % — над текущим (‹ ⊞ › над корешком), при зуме — всё окно; в движении ряда гаснет -->
        <div class="frame" class:away style:left={zoomed ? '0' : `${curLeft}px`} style:width={zoomed ? '100%' : `${place.width}px`}>
          <!-- Режим фона: вместо «Раскладка: …» — полоса режима -->
          {#if bgmode.cur}<BackdropBar />{:else}<LayoutBar spread={store.spread} show={hovered} compact={zoomed} />{/if}
        </div>
      {/if}
    </div>

    <!-- Окно холста под полосой: ряд разворотов обрезается им и не заходит на полосу -->
    <div class="viewport">
      <SpreadSlots settings={s} {width} {spreadW} {w} {aspect} fit={fitPlace} {place} {zoomed} onhover={(on) => (hovSpread = on)} />
    </div>

    {#if store.spread && modes.rulers && prefs.can('rulers')}
      <Rulers stage={el} left={curLeft} top={place.top} width={spreadW} {w} {h} />
    {/if}

    <!-- Панели правки: вне окна холста — при зуме, панораме и листании стоят -->
    <EditRail />
    {#if !modes.details}<Inspector />{/if}
    {#if store.spread}<ZoomBadge />{/if}
    <Marquee stage={el} bind:this={marq} />
  {/if}
</div>

<style>
  /* panels.md: полоса, окно холста, лента и карточка — дети .stage; ряд разворотов стоит в окне холста
     по placement (целые px) */
  .stage {
    position: relative;
    flex: 1;
    min-height: 0;
    overflow: hidden;
    isolation: isolate;
  }
  .stage.grab,
  .stage.grab :global(*) {
    cursor: grab !important;
  }
  .stage.grabbing,
  .stage.grabbing :global(*) {
    cursor: grabbing !important;
  }
  .topbar {
    position: absolute;
    left: 0;
    right: 0;
    top: 0;
    height: 32px; /* = BAR */
    z-index: 30;
    background: #1b1b1d;
  }
  .frame {
    position: absolute;
    top: 0;
    bottom: 0;
    transition: opacity 0.15s;
  }
  .frame.away {
    opacity: 0;
    pointer-events: none;
  }
  /* Свои z-index ячеек и мест остаются внутри окна холста */
  .viewport {
    position: absolute;
    left: 0;
    right: 0;
    top: 32px;
    bottom: 0;
    z-index: 0;
    overflow: hidden;
    isolation: isolate;
  }
</style>
