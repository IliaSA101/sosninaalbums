<script lang="ts">
  import { store, Api } from '../../base/core/store.svelte'
  import { placesel } from '../places/placesel.svelte'
  import { edit } from '../text/edit.svelte'
  import { photoui } from './photoui.svelte'
  import { straighten } from './straighten.svelte'
  import { prefs } from '../../base/prefs/prefs.svelte'
  import { menuContext, photoMenu, type MenuId } from './photomenu'
  import { LAYER_OPS, type LayerOp } from '../places/minipanel'
  import { relinkFolder } from '../../panels/library/relink'
  import { decor } from '../../panels/decor/decor.svelte'
  import { sticker } from '../sticker/sticker.svelte'
  import { errText } from '../../base/core/errtext'
  import { scopeLabel, scopesFor } from '../backdrop/backdrop'
  import { setBackdrop } from '../backdrop/backdropact'
  import { COVER_SCOPE_LABEL, layoutScopes } from '../cover/cover'
  import { isCover, isSheet } from '../../base/geom/sheet'
  import type { BackdropScope } from '../../base/core/types'

  const sheet = $derived(isSheet(store.view.settings))
  // На обложке (032) фон ставят в области её раскладки, названия — сторонки; в листе (031) — только «Весь лист».
  const cover = $derived(isCover(store.spread))
  const scopes = $derived(cover ? layoutScopes(store.cover?.layout) : scopesFor(sheet))
  const label = (sc: BackdropScope) => (cover ? COVER_SCOPE_LABEL[sc] : scopeLabel(sc, sheet))

  // Меню правого щелчка по фото (спека 008): своё, работает и в браузере. Пункты — photo/photomenu.ts.
  // Esc и щелчок мимо закрывают; окно держится внутри экрана.
  // Выделены одни стикеры (048) — меню стикера: «Заменить…», «Зеркало», «Слой», «Удалить».
  const items = $derived(
    photoMenu(
      menuContext(store.spread, placesel.selected, store.missing, placesel.texts.length, placesel.selectedStickers.length),
      prefs.can,
    ),
  )
  const GAP = 8 // отступ от края окна, px

  // «Слой ▸» и «На фон ▸» раскрываются списком под пунктом (не боковым окном: не упирается в край экрана).
  let subOpen = $state<MenuId | null>(null)
  let box = $state<HTMLElement>()
  let w = $state(0)
  let h = $state(0)
  const pos = $derived.by(() => {
    const m = photoui.menu
    if (!m) return { x: 0, y: 0 }
    return {
      x: Math.max(GAP, Math.min(m.x, window.innerWidth - w - GAP)),
      y: Math.max(GAP, Math.min(m.y, window.innerHeight - h - GAP)),
    }
  })

  // Выделение пропало (правка, смена разворота) — пунктам не на что опираться.
  $effect(() => {
    if (photoui.menu && items.length === 0) photoui.close()
  })
  // Новое открытие меню — подменю свёрнуты.
  $effect(() => {
    if (!photoui.menu) subOpen = null
  })
  // Фокус на первый пункт: стрелки и Enter работают сразу.
  $effect(() => {
    if (box) box.querySelector<HTMLElement>('button')?.focus()
  })

  function key(e: KeyboardEvent) {
    if (!photoui.menu) return
    // Пока меню открыто, горячие клавиши приложения не работают (как в PhotoPreview);
    // Enter и Пробел отрабатывает кнопка по умолчанию.
    e.stopPropagation()
    if (e.key === 'Escape') {
      e.preventDefault()
      photoui.close()
    } else if ((e.key === 'ArrowDown' || e.key === 'ArrowUp') && box) {
      e.preventDefault()
      const bs = [...box.querySelectorAll<HTMLElement>('button')]
      const at = bs.indexOf(document.activeElement as HTMLElement)
      bs[(at + (e.key === 'ArrowDown' ? 1 : -1) + bs.length) % bs.length]?.focus()
    }
  }

  function outside(e: PointerEvent) {
    if (photoui.menu && box && !box.contains(e.target as Node)) photoui.close()
  }

  const photos = () => placesel.selected.filter((b) => b.photo).map((b) => b.photo)

  async function copyNames() {
    // Только найденные: у пропавших имени во фронте нет.
    const names = photos()
      .filter((id) => !store.missing.includes(id))
      .map((id) => store.byId.get(id)?.name)
      .filter((n): n is string => !!n)
    if (names.length === 0) return
    try {
      await navigator.clipboard.writeText(names.join('\n'))
      store.notify(names.length > 1 ? 'Имена файлов скопированы' : 'Имя файла скопировано', 'info')
    } catch {
      // Текст отказа буфера приходит от браузера по-английски — свой.
      store.notify('Не удалось скопировать в буфер обмена', 'error')
    }
  }

  async function explorer(id: string) {
    try {
      await Api.ShowPhotoInExplorer(id)
    } catch (e) {
      store.notify(errText(e), 'error')
    }
  }

  function layer(op: LayerOp) {
    photoui.close(true)
    placesel.layer(op)
  }

  function toBackdrop(scope: BackdropScope) {
    const s = store.spread
    const first = photos()[0]
    photoui.close(true)
    if (s && first) setBackdrop(s.id, scope, first)
  }

  function act(id: MenuId) {
    if (id === 'layer' || id === 'backdrop') {
      subOpen = subOpen === id ? null : id
      return
    }
    const first = photos()[0]
    photoui.close(true)
    ACTIONS[id]?.(first)
  }

  // «Заменить…» у стикера: то же окно «Стикеры», что и в карточке.
  function replaceSticker() {
    const t = sticker.target()
    if (t) decor.show('sticker', t.spread, 'spread', t.id)
  }

  // Действие пункта; first — первое выделенное фото (у части пунктов оно не нужно).
  const ACTIONS: Partial<Record<MenuId, (first: string) => void>> = {
    preview: (first) => (photoui.preview = first),
    copy: () => void copyNames(),
    explorer: (first) => void explorer(first),
    relink: () => void relinkFolder(),
    fitPhoto: () => placesel.arrange('fitPhoto'),
    turnCw: () => placesel.turn(1),
    turnCcw: () => placesel.turn(-1),
    straighten: (first) => straighten.startPhoto(first),
    flipH: () => placesel.flip('h'),
    flipV: () => placesel.flip('v'),
    contain: () => placesel.setFit(true),
    cover: () => placesel.setFit(false),
    round: () => placesel.setShape(true),
    rect: () => placesel.setShape(false),
    resetFrame: (first) => void store.run(Api.ResetFrame(first)),
    caption: (first) => edit.editCaption(first),
    replace: replaceSticker,
    delete: () => placesel.deleteSelected(),
  }
</script>

<svelte:window onkeydowncapture={key} onpointerdowncapture={outside} onresize={() => photoui.close()} onblur={() => photoui.close()} />

{#if photoui.menu && items.length > 0}
  <div
    bind:this={box}
    bind:clientWidth={w}
    bind:clientHeight={h}
    class="menu"
    style:left="{pos.x}px"
    style:top="{pos.y}px"
    style:visibility={h ? undefined : 'hidden'}
    role="menu"
    tabindex="-1"
    data-chrome
    data-text-ui
    oncontextmenu={(e) => e.preventDefault()}
  >
    {#each items as it (it.id)}
      {#if it.sep}<div class="sep" role="separator"></div>{/if}
      <button
        class:danger={it.danger}
        role="menuitem"
        aria-haspopup={it.sub ? 'menu' : undefined}
        aria-expanded={it.sub ? subOpen === it.id : undefined}
        onclick={() => act(it.id)}
      >
        <span>{it.label}</span>
        {#if it.hint}<kbd>{it.hint}</kbd>{/if}
        {#if it.sub}<kbd>{subOpen === it.id ? '▾' : '▸'}</kbd>{/if}
      </button>
      {#if it.sub && it.id === 'layer' && subOpen === 'layer'}
        {#each LAYER_OPS as lo (lo.op)}
          <button class="sub" role="menuitem" onclick={() => layer(lo.op)}><span>{lo.label}</span></button>
        {/each}
      {:else if it.sub && it.id === 'backdrop' && subOpen === 'backdrop'}
        {#each scopes as sc (sc)}
          <button class="sub" role="menuitem" onclick={() => toBackdrop(sc)}><span>{label(sc)}</span></button>
        {/each}
      {/if}
    {/each}
  </div>
{/if}

<style>
  .menu {
    position: fixed;
    z-index: 70;
    min-width: 200px;
    padding: 4px;
    box-sizing: border-box;
    background: rgba(30, 30, 32, 0.97);
    border: 1px solid var(--line);
    border-radius: 6px;
    box-shadow: 0 6px 20px rgba(0, 0, 0, 0.5);
    font-size: 13px;
    user-select: none;
  }
  button {
    width: 100%;
    height: 28px;
    padding: 0 10px;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 16px;
    text-align: left;
    white-space: nowrap;
    background: transparent;
    color: var(--text);
    border-radius: 4px;
  }
  button:hover,
  button:focus-visible {
    background: #38383c;
    color: #fff;
  }
  button.danger:hover,
  button.danger:focus-visible {
    color: #ff8a8a;
  }
  button.sub {
    padding-left: 24px;
  }
  kbd {
    font: inherit;
    font-size: 11px;
    color: var(--muted);
  }
  .sep {
    height: 1px;
    margin: 4px 2px;
    background: var(--line);
  }
</style>
