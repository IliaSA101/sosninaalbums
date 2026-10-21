<script lang="ts">
  import { primaryPress } from '../../base/core/platform'
  import { tick, untrack } from 'svelte'
  import { store } from '../../base/core/store.svelte'
  import { prefs } from '../../base/prefs/prefs.svelte'
  import { gallery } from '../gallery/gallery.svelte'
  import { drag } from '../../base/drag/drag.svelte'
  import { modes } from '../../base/prefs/view.svelte'
  import { scanText } from '../folders/scan'
  import { allInAlbum, allInAlbumText } from './freephotos'
  import { libfilter } from '../marks/libfilter.svelte'
  import { keymap } from '../../base/keys/keymap.svelte'
  import { binsel } from '../bins/binsel.svelte'
  import { binsOf, emptyBinText, filterBin, hiddenBinText } from '../bins/bins'
  import { folderfilter } from '../folders/folderfilter.svelte'
  import { isThumbWheel, thumbWheelAcc } from './thumbzoom'
  import { LIB_DEF, PAD_X, SPLITTER, anchorTop, colsOf, keepScroll, libFloor, libWindow, photoAt } from './libwindow'
  import { watchSize } from '../../base/ui/size'
  import LibraryBar from './LibraryBar.svelte'
  import Thumb from './Thumb.svelte'

  // Высота библиотеки: по умолчанию LIB_DEF, меняется сплиттером и запоминается.
  // Сохранённая высота (h) — желаемая: при нехватке окна CSS сжимает ленту до одного ряда плиток (D15, 089),
  // настройка при этом не меняется, и в большем окне лента снова прежняя.
  const KEY = 'libraryHeight'
  const MIN_H = 140
  let wh = $state(window.innerHeight)
  let box = $state<HTMLElement>()
  let h = $state(load())
  // Пока тянут сплиттер, высота без анимации, иначе лента отстаёт от мыши.
  let resizing = $state(false)
  // Крупный просмотр и открытая галерея раскладок сворачивают ленту: галерея
  // отнимает высоту у разворота, и без этого он сжимается до нечитаемого.
  const collapsed = $derived(modes.big || !!gallery.cur)
  const eff = $derived(Math.max(MIN_H, Math.min(h, Math.max(MIN_H, wh * 0.6))))
  // Ниже этой высоты лента не сжимается: один полный ряд плиток (но не выше заданной).
  const floor = $derived(libFloor(modes.thumbSize, eff))

  function load(): number {
    try {
      const v = Number(localStorage.getItem(KEY))
      return v > 0 ? v : LIB_DEF
    } catch {
      return LIB_DEF
    }
  }

  function split(e: PointerEvent) {
    if (!primaryPress(e)) return
    const el = e.currentTarget as HTMLElement
    const y0 = e.clientY
    // Начало жеста — фактическая высота (лента могла быть сжата окном), а не заданная.
    const h0 = box?.offsetHeight ?? eff
    el.setPointerCapture(e.pointerId)
    resizing = true
    const move = (ev: PointerEvent) => {
      h = Math.max(MIN_H, Math.min(h0 + (y0 - ev.clientY), Math.max(MIN_H, wh * 0.6)))
    }
    const up = () => {
      el.removeEventListener('pointermove', move)
      el.removeEventListener('pointerup', up)
      el.removeEventListener('pointercancel', up)
      resizing = false
      try {
        localStorage.setItem(KEY, String(Math.round(h)))
      } catch {
        // Без хранилища высота просто не запомнится.
      }
    }
    el.addEventListener('pointermove', move)
    el.addEventListener('pointerup', up)
    el.addEventListener('pointercancel', up)
  }

  // Ctrl/Alt/Cmd+колесо над лентой — размер миниатюр (053). Дельты копятся до кадра, как зум холста.
  let grid = $state<HTMLElement>()
  const acc = thumbWheelAcc()
  let wx = 0
  let wy = 0
  let raf = 0
  function wheel(e: WheelEvent) {
    if (!isThumbWheel(e)) return
    e.preventDefault() // иначе масштабируется страница
    if (drag.active) return
    const first = acc.push(modes.thumbSize, e)
    wx = e.clientX
    wy = e.clientY
    if (first && !raf) raf = requestAnimationFrame(() => void applyZoom())
  }
  // Окно рядов (073): в DOM только видимые ряды ± запас, остальное — распорки. Размер сетки — watchSize.
  let gw = $state(0)
  let gh = $state(0)
  let st = $state(0)
  const win = $derived(libWindow({ n: store.visible.length, width: gw, height: gh, scrollTop: st, size: modes.thumbSize }))
  const shown = $derived(store.visible.slice(win.r0 * win.cols, win.r1 * win.cols))

  // Список ленты сменился (файлы пропали или добавились, 088): верхнее видимое фото остаётся на месте.
  // Следим только за списком; ширина, размер и прокрутка читаются без подписки (зум ведёт applyZoom).
  let prevIds: string[] = []
  $effect.pre(() => {
    const ids = store.visible.map((p) => p.id)
    const prev = prevIds
    prevIds = ids
    const to = untrack(() => (grid ? keepScroll(prev, ids, grid.scrollTop, grid.clientWidth, modes.thumbSize) : null))
    if (to === null) return
    // Высота содержимого должна стать новой, иначе браузер подрежет прокрутку.
    void tick().then(() => {
      if (!grid || Math.abs(grid.scrollTop - to) < 0.5) return
      grid.scrollTop = to
      st = grid.scrollTop
    })
  })

  // Якорь зума — индекс фото под курсором (элемент мог уйти из окна); вне сетки — верхний видимый ряд.
  function anchor(g: HTMLElement): { index: number; cy: number; fy: number } | null {
    const r = g.getBoundingClientRect()
    const inside = wx >= r.left && wx < r.right && wy >= r.top && wy < r.bottom
    const cy = inside ? wy - r.top : 0
    const at = photoAt(store.visible.length, g.clientWidth, modes.thumbSize, inside ? wx - r.left : PAD_X, g.scrollTop + cy)
    return at && { ...at, cy }
  }
  async function applyZoom() {
    raf = 0
    const n = acc.take()
    if (!grid || n === modes.thumbSize) return
    const a = anchor(grid)
    modes.setThumb(n)
    await tick() // высота содержимого уже новая, иначе браузер подрежет прокрутку
    if (!a || !grid) return
    // Прокрутка считается из индекса и нового размера; окно обновляем сразу, а не по событию scroll.
    grid.scrollTop = anchorTop(a.index, colsOf(grid.clientWidth, n), n, a.cy, a.fy)
    st = grid.scrollTop
  }
  // Свой слушатель с passive: false — иначе preventDefault у колеса не сработает.
  function wheelOn(node: HTMLElement) {
    node.addEventListener('wheel', wheel, { passive: false })
    return {
      destroy() {
        node.removeEventListener('wheel', wheel)
        if (raf) cancelAnimationFrame(raf)
        raf = 0
        acc.reset()
      },
    }
  }
</script>

<svelte:window bind:innerHeight={wh} />

<!-- Свёрнутая лента не размонтируется: выделение, автосборка и прогрев миниатюр продолжаются. -->
<div
  class="wrap"
  class:resizing
  style:flex-basis="{collapsed ? 0 : eff + SPLITTER}px"
  style:min-height="{collapsed ? 0 : floor + SPLITTER}px"
  inert={collapsed}
>
  <div
    class="splitter"
    role="separator"
    aria-orientation="horizontal"
    title="Потяните, чтобы изменить высоту ленты фото"
    onpointerdown={split}
  ></div>
  <div
    class="library"
    bind:this={box}
    use:wheelOn
    class:over={drag.over === 'library'}
    style:min-height="{floor}px"
    data-drop="library"
    data-guide="library"
  >
    <LibraryBar />

    <div
      class="grid"
      bind:this={grid}
      style:--size="{modes.thumbSize}px"
      {@attach (g) => watchSize(g, (w, h) => ((gw = w), (gh = h)))}
      onscroll={() => (st = grid?.scrollTop ?? 0)}
    >
      {#if win.padTop > 0}<div class="spacer" style:height="{win.padTop}px"></div>{/if}
      {#each shown as p (p.id)}
        <Thumb {p} />
      {/each}
      {#if win.padBottom > 0}<div class="spacer" style:height="{win.padBottom}px"></div>{/if}
      {#if store.visible.length === 0 && store.loaded}
        <div class="hint">
          {#if store.scan.active && store.photos.length === 0}
            {scanText(store.scan)}
          {:else if store.view.folders.length === 0}
            Нажмите «+ Папка с фото» или перетащите папку прямо в окно программы
          {:else if folderfilter.allHidden(store.view.folders)}
            Все папки скрыты
            <button class="reset" onclick={() => folderfilter.showAll()}>Показать все</button>
          {:else if binsel.shown && store.inBin.length === 0 && filterBin(store.photos, binsel.shown, binsel.index).length > 0}
            {hiddenBinText(binsel.curName(binsOf(store.view)))}
            <button class="reset" onclick={() => folderfilter.showAll()}>Показать все папки</button>
          {:else if binsel.shown && store.inBin.length === 0}
            {emptyBinText(binsel.curName(binsOf(store.view)), keymap.hint('binToggle'))}
            <button class="reset" onclick={() => binsel.showAll()}>Показать все фото</button>
          {:else if allInAlbum(store.filter, store.free.length, store.inBin.length)}
            {allInAlbumText(folderfilter.active, binsel.curName(binsOf(store.view)))}
            <button class="reset" onclick={() => (store.filter = 'all')}>Показать все</button>
          {:else if prefs.can('marks') && libfilter.active && store.inBin.length > 0}
            Ни одно фото не подходит под фильтр
            <button class="reset" onclick={() => libfilter.reset()}>Сбросить фильтр</button>
          {:else}
            Здесь пусто
          {/if}
        </div>
      {/if}
    </div>
    {#if drag.over === 'library'}
      <div class="drop-hint">Отпустите, чтобы убрать фото из альбома</div>
    {/if}
  </div>
</div>

<style>
  /* Основа flex-basis (eff + сплиттер) задана в разметке. Сжимается раньше холста: flex-shrink на порядки
     больше, чем у Stage, а min-height (один ряд плиток) держит нижний предел; потом уступает холст */
  .wrap {
    flex: 0 1 auto;
    flex-shrink: 10000;
    display: flex;
    flex-direction: column;
    overflow: hidden;
    transition:
      flex-basis var(--move),
      min-height var(--move);
  }
  .wrap.resizing {
    transition: none;
  }
  .splitter {
    flex: none;
    height: 6px; /* = SPLITTER */
    cursor: row-resize;
    background: var(--panel2);
    touch-action: none;
  }
  .splitter:hover {
    background: #3a3a3e;
  }
  /* Высота ленты — от .wrap и от содержимого не зависит: пустое состояние «Все папки скрыты» её не меняет.
     min-height (в разметке) = нижний предел: у свёрнутой ленты сетка не схлопывается, обрезает .wrap */
  .library {
    position: relative;
    flex: 1 1 0;
    overflow: hidden;
    display: flex;
    flex-direction: column;
    background: var(--panel);
    border-top: 1px solid var(--line);
  }
  .library.over {
    box-shadow: inset 0 0 0 3px var(--accent);
  }
  .grid {
    flex: 1;
    min-height: 0;
    overflow-y: auto;
    display: flex;
    flex-wrap: wrap;
    align-content: flex-start;
    gap: 16px;
    padding: 12px 16px;
  }
  /* Заменяет пропущенные ряды окна: на всю строку, высота — по числу рядов (libwindow.ts) */
  .spacer {
    flex: 0 0 100%;
  }
  .hint {
    color: var(--muted);
    padding: 12px 30px; /* невысокая подсказка: влезает и в минимальную ленту, прокрутки нет */
    margin: auto;
  }
  .hint .reset {
    display: block;
    margin: 10px auto 0;
  }
  .drop-hint {
    position: absolute;
    inset: 44px 0 0;
    display: flex;
    justify-content: center;
    align-items: flex-start; /* вверху, чтобы призрак под курсором не закрывал */
    padding-top: 14px;
    background: rgba(0, 0, 0, 0.55);
    color: var(--accent);
    font-size: 18px;
    pointer-events: none;
  }
</style>
