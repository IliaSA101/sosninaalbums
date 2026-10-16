<script lang="ts">
  import { store } from '../../base/core/store.svelte'
  import { prefs } from '../../base/prefs/prefs.svelte'
  import { gallery } from '../gallery/gallery.svelte'
  import { drag } from '../../base/drag/drag.svelte'
  import { modes } from '../../base/prefs/view.svelte'
  import { scanText } from './scan'
  import { allInAlbum } from './freephotos'
  import { libfilter } from '../marks/libfilter.svelte'
  import LibraryBar from './LibraryBar.svelte'
  import Thumb from './Thumb.svelte'

  // Высота библиотеки: по умолчанию 240px, меняется сплиттером и запоминается.
  const KEY = 'libraryHeight'
  const MIN_H = 140
  let wh = $state(window.innerHeight)
  let h = $state(load())
  // Пока тянут сплиттер, высота без анимации, иначе лента отстаёт от мыши.
  let resizing = $state(false)
  const SPLITTER = 6
  // Крупный просмотр и открытая галерея раскладок сворачивают ленту: галерея
  // отнимает высоту у разворота, и без этого он сжимается до нечитаемого.
  const collapsed = $derived(modes.big || !!gallery.cur)
  const eff = $derived(Math.max(MIN_H, Math.min(h, Math.max(MIN_H, wh * 0.6))))

  function load(): number {
    try {
      const v = Number(localStorage.getItem(KEY))
      return v > 0 ? v : 240
    } catch {
      return 240
    }
  }

  function split(e: PointerEvent) {
    if (e.button !== 0) return
    const el = e.currentTarget as HTMLElement
    const y0 = e.clientY
    const h0 = eff
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
</script>

<svelte:window bind:innerHeight={wh} />

<!-- Свёрнутая лента не размонтируется: выделение, автосборка и прогрев миниатюр продолжаются. -->
<div class="wrap" class:resizing style:height="{collapsed ? 0 : eff + SPLITTER}px" inert={collapsed}>
  <div
    class="splitter"
    role="separator"
    aria-orientation="horizontal"
    title="Потяните, чтобы изменить высоту ленты фото"
    onpointerdown={split}
  ></div>
  <div class="library" class:over={drag.over === 'library'} style:height="{eff}px" data-drop="library" data-guide="library">
    <LibraryBar />

    <div class="grid" style:--size="{store.thumbSize}px">
      {#each store.visible as p (p.id)}
        <Thumb {p} />
      {:else}
        {#if store.loaded}
          <div class="hint">
            {#if store.scan.active && store.photos.length === 0}
              {scanText(store.scan)}
            {:else if store.view.folders.length === 0}
              Нажмите «+ Папка с фото» или перетащите папку прямо в окно программы
            {:else if allInAlbum(store.filter, store.free.length, store.photos.length)}
              Все фото уже в альбоме
              <button class="reset" onclick={() => (store.filter = 'all')}>Показать все</button>
            {:else if prefs.can('marks') && libfilter.active && store.photos.length > 0}
              Ни одно фото не подходит под фильтр
              <button class="reset" onclick={() => libfilter.reset()}>Сбросить фильтр</button>
            {:else}
              Здесь пусто
            {/if}
          </div>
        {/if}
      {/each}
    </div>
    {#if drag.over === 'library'}
      <div class="drop-hint">Отпустите, чтобы убрать фото из альбома</div>
    {/if}
  </div>
</div>

<style>
  .wrap {
    flex: none;
    display: flex;
    flex-direction: column;
    overflow: hidden;
    transition: height var(--move);
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
  .library {
    position: relative;
    flex: none;
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
    overflow-y: auto;
    display: flex;
    flex-wrap: wrap;
    align-content: flex-start;
    gap: 16px;
    padding: 12px 16px;
  }
  .hint {
    color: var(--muted);
    padding: 30px;
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
