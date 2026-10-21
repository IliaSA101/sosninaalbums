<script lang="ts">
  import { tick } from 'svelte'
  import Section from './Section.svelte'
  import AlbumInfo from './AlbumInfo.svelte'
  import SettingsForm from './SettingsForm.svelte'
  import SpreadGrid from './SpreadGrid.svelte'
  import Inspector from '../../editor/inspector/Inspector.svelte'
  import PhotoAbout from '../../editor/photoprops/PhotoAbout.svelte'
  import { libPhotoOf } from '../../editor/photoprops/aboutload.svelte'
  import { store } from '../../base/core/store.svelte'
  import { newSheetSize, pageSize, unitOf } from '../../base/geom/sheet'
  import { modes } from '../../base/prefs/view.svelte'
  import { prefs } from '../../base/prefs/prefs.svelte'
  import { panels } from '../../editor/inspector/panelmode.svelte'
  import { placesel } from '../../editor/places/placesel.svelte'
  import { needScroll, reachableTop, revealDelta } from './reveal'

  const s = $derived(store.view.settings)
  // Формат страницы и лист альбома (разворот или отдельный лист) — через sheet.ts.
  const page = $derived(pageSize(s))
  const sheet = $derived(newSheetSize(s))
  const unit = $derived(unitOf(s))
  // Без лишних нулей: 21.5, а не 21.50.
  const num = (n: number) => String(Math.round(n * 100) / 100)

  // Раздел «Фото из ленты» (084): одно выделенное в ленте фото при открытой панели.
  const libId = $derived(libPhotoOf(store.selectedIds, modes.details, store.selectedCell ?? ''))

  let scroller: HTMLDivElement

  // Кнопка «Формат» и ссылки «в настройках альбома»: прокрутить к разделу, даже если панель уже открыта.
  // Панель и раздел раскрываются анимацией (--move, ~1 с с учётом открытия панели), scrollHeight и offsetTop
  // растут на ходу, поэтому цель пересчитывается каждый кадр: прокрутка перезапускается только когда
  // достижимая цель сменилась (без мигания), и так до REVEAL_MS. Ручная прокрутка пользователя обрывает слежение.
  const REVEAL_MS = 1400
  $effect(() => {
    const r = modes.reveal
    if (!r) return
    let raf = 0
    let last = NaN
    const t0 = performance.now()
    const stop = () => cancelAnimationFrame(raf)
    const frame = () => {
      const el = scroller?.querySelector<HTMLElement>(`[data-section="${r.name}"]`)
      if (el) {
        const top = reachableTop(el.offsetTop, scroller.scrollHeight, scroller.clientHeight)
        if (top !== last) {
          last = top
          scroller.scrollTo({ top, behavior: 'smooth' })
        }
      }
      if (performance.now() - t0 < REVEAL_MS) raf = requestAnimationFrame(frame)
    }
    tick().then(frame)
    scroller?.addEventListener('wheel', stop, { passive: true })
    scroller?.addEventListener('pointerdown', stop)
    return () => {
      stop()
      scroller?.removeEventListener('wheel', stop)
      scroller?.removeEventListener('pointerdown', stop)
    }
  })

  // Подвести элемент в поле зрения своей прокруткой: scrollIntoView сдвинул бы и aside (overflow: hidden).
  function reveal(el: HTMLElement) {
    if (!scroller) return
    const a = scroller.getBoundingClientRect()
    const b = el.getBoundingClientRect()
    scroller.scrollTop += revealDelta(b.top - a.top, b.bottom - a.top, scroller.clientHeight)
  }

  // Новое выделение, а заголовок карточки выше видимой части — к началу панели. Снятие выделения не в счёт:
  // смену разворота ведёт «Все развороты» (SpreadGrid), им не мешаем.
  const selKey = $derived(`${panels.mode}|${placesel.refs.join(',')}|${placesel.texts.join(',')}|${placesel.stickers.join(',')}`)
  let lastKey = ''
  $effect(() => {
    const k = selKey
    if (k === lastKey) return
    lastKey = k
    if (!modes.details || panels.mode === 'idle' || !scroller) return
    const card = scroller.querySelector<HTMLElement>('[data-card]')
    if (card && needScroll(card.offsetTop, scroller.scrollTop)) scroller.scrollTo({ top: 0, behavior: 'smooth' })
  })
</script>

<!-- Ширина анимируется, содержимое фиксированной ширины 400 px: текст не перетекает. -->
<aside class="details" class:open={modes.details} inert={!modes.details} aria-label="Панель деталей">
  <div class="scroll" bind:this={scroller}>
    <!-- Карточка свойств выделенного: первый блок прокрутки, сворачивается (028 Б); на виньетке в ней раздел «Виньетка» -->
    {#if modes.details}<Inspector docked {reveal} />{/if}
    {#if libId}
      <Section name="photo" title="Фото из ленты"><div data-guide="photo-section"><PhotoAbout id={libId} head /></div></Section>
    {/if}
    <div class="format">
      <div class="name">{num(page.w / 10)}×{num(page.h / 10)}</div>
      <div class="sub">
        {sheet.mid > 0 ? `разворот ${num(sheet.w / 10)}×${num(sheet.h / 10)} см` : 'один лист без корешка'}, {s.dpi} dpi
      </div>
    </div>
    <Section name="info" title="Сведения об альбоме"><AlbumInfo /></Section>
    <Section name="settings" title="Настройки альбома"><SettingsForm /></Section>
    {#if prefs.can('allSpreads')}<Section name="grid" title={unit.every}><SpreadGrid /></Section>{/if}
  </div>
</aside>

<style>
  .details {
    flex: none;
    display: flex;
    flex-direction: column;
    width: 0;
    overflow: hidden;
    background: var(--panel);
    /* Линия слева тенью, а не рамкой: при ширине 0 рамка осталась бы видна */
    box-shadow: inset 1px 0 0 var(--line);
    transition: width var(--move);
  }
  .details.open {
    width: 400px;
  }
  .scroll {
    position: relative; /* offsetTop разделов — от начала прокрутки */
    width: 400px;
    flex: 1;
    min-height: 0;
    overflow-x: hidden;
    overflow-y: auto;
    padding-left: 1px;
  }
  .format {
    padding: 18px 16px 16px;
  }
  .name {
    font-size: 30px;
    font-weight: 600;
    line-height: 1.1;
  }
  .sub {
    margin-top: 4px;
    font-size: 13px;
    color: var(--muted);
  }
</style>
