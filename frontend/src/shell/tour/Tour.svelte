<script lang="ts">
  import { untrack } from 'svelte'
  import { jobs } from '../../base/core/jobs.svelte'
  import { store } from '../../base/core/store.svelte'
  import { drag } from '../../base/drag/drag.svelte'
  import { modalStack } from '../../base/modal/modalstack.svelte'
  import { prefs } from '../../base/prefs/prefs.svelte'
  import { spot } from '../guide/spotstate.svelte'
  import TourCard from './TourCard.svelte'
  import { hasPhotoOnSpread, POPUP_SEL, shouldAutostart } from './tour'
  import { tour } from './tourstate.svelte'

  // Слой тура (спека 113, вид A): затемнение с вырезом по кольцу и ловушка мыши вне выреза. Тур не модальный:
  // в modalStack не встаёт, клавиши программы работают; вырез пропускает события, мимо него щелчок ничего не делает.

  // Автозапуск: после закрытия стартового окна и других окон, когда альбом загружен.
  $effect(() => {
    const go = shouldAutostart({
      due: prefs.tourDue,
      startOpen: prefs.startOpen,
      modal: modalStack.any(),
      loaded: store.loaded,
      opening: jobs.openShown,
      running: tour.open || tour.started,
    })
    if (go) untrack(() => tour.start('auto'))
  })

  // Место ищется при входе в шаг, при смене места по состоянию альбома и когда окно закрылось; под окном кольцо снимается.
  $effect(() => {
    if (!tour.open) return
    const blocked = tour.blocked
    void tour.key
    untrack(() => (blocked ? tour.release() : void tour.place()))
  })

  // «Показать» справочника погасло — кольцо возвращается к шагу.
  $effect(() => {
    const cur = spot.cur
    untrack(() => tour.onSpot(cur))
  })

  // Разовое продолжение: тур пройден без фото на разворотах, а теперь они есть. Чужой тост не вытесняем:
  // эффект читает store.toast и перезапускается, когда тост исчез.
  $effect(() => {
    if (store.toast || !prefs.tourMore || tour.open || modalStack.any() || !store.loaded || !hasPhotoOnSpread(tour.ctx)) return
    untrack(() => tour.offerMore())
  })

  // Меню «⋯» и поповеры живут в body: следим за ними, пока тур открыт.
  $effect(() => {
    if (!tour.open) return
    const sync = () => (tour.popup = !!document.querySelector(POPUP_SEL))
    const mo = new MutationObserver(sync)
    mo.observe(document.body, { childList: true })
    sync()
    return () => mo.disconnect()
  })

  const ring = $derived(tour.ring)

  // Вне выреза всё гасим, но фокус из карточки не уводим (mousedown), иначе стрелки и Esc перестали бы работать.
  const stop = (e: Event) => e.preventDefault()
</script>

{#if tour.open && !tour.hidden}
  <div class="layer" class:pass={drag.active || tour.passThrough} class:lit={tour.passThrough} data-tour-layer>
    {#if ring}
      <div
        class="hole"
        data-tour-dim
        style:left="{ring.left}px"
        style:top="{ring.top}px"
        style:width="{ring.width}px"
        style:height="{ring.height}px"
      ></div>
      <!-- Четыре полосы вокруг выреза: сам вырез остаётся открытым для мыши -->
      <div class="trap" role="presentation" onmousedown={stop} oncontextmenu={stop} style="left:0;right:0;top:0;height:{ring.top}px"></div>
      <div
        class="trap"
        role="presentation"
        onmousedown={stop}
        oncontextmenu={stop}
        style="left:0;right:0;top:{ring.top + ring.height}px;bottom:0"
      ></div>
      <div
        class="trap"
        role="presentation"
        onmousedown={stop}
        oncontextmenu={stop}
        style="left:0;width:{ring.left}px;top:{ring.top}px;height:{ring.height}px"
      ></div>
      <div
        class="trap"
        role="presentation"
        onmousedown={stop}
        oncontextmenu={stop}
        style="left:{ring.left + ring.width}px;right:0;top:{ring.top}px;height:{ring.height}px"
      ></div>
    {:else}
      <div class="dim" data-tour-dim></div>
      <div class="trap" role="presentation" onmousedown={stop} oncontextmenu={stop} style="inset:0"></div>
    {/if}
  </div>
  <TourCard />
{/if}

<style>
  /* Ниже кольца (z 90) и карточки (91), выше окна программы */
  .layer {
    position: fixed;
    inset: 0;
    z-index: 89;
    pointer-events: none;
  }
  .dim {
    position: absolute;
    inset: 0;
    background: rgba(0, 0, 0, 0.45);
  }
  .hole {
    position: absolute;
    border-radius: 8px;
    box-shadow: 0 0 0 200vmax rgba(0, 0, 0, 0.45);
  }
  .trap {
    position: absolute;
    pointer-events: auto;
    cursor: default;
  }
  /* Пока тащат фото из ленты на разворот или открыто окно из выреза (галерея, меню «⋯»), ловушка пропускает всё:
     цель броска и пункты лежат вне выреза */
  .pass .trap {
    pointer-events: none;
  }
  /* Открытое под слоем (меню z 70) не затемняем: оно и есть то, что нажимают */
  .lit .dim {
    display: none;
  }
  .lit .hole {
    box-shadow: none;
  }
</style>
