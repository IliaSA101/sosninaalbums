<script lang="ts">
  import { tick, untrack } from 'svelte'
  import type { BackdropScope, SpreadView } from '../../base/core/types'
  import { eventChord } from '../../base/keys/chord'
  import { keymap } from '../../base/keys/keymap.svelte'
  import { isMac, primaryPress } from '../../base/core/platform'
  import { modalStack, Z_MODAL } from '../../base/modal/modalstack.svelte'
  import { cancelOnUnmount } from '../../base/ui/imgcancel'
  import Icon from '../../base/ui/Icon.svelte'
  import { PreviewLoupe } from '../../editor/photo/loupe.svelte'
  import { ZOOM_STEP } from '../../editor/zoom/zoom'
  import { decor } from '../decor/decor.svelte'
  import { peek } from './peek.svelte'
  import { BUILTIN_PACK } from './browse'
  import { peekCaption, peekGuess, peekHint, peekSrc, thumbSrc } from './peekaddr'

  // Крупный просмотр плитки окон оформления (133 U2), образец — PhotoPreview: вписано, шашечка, лупа, подпись, ✕.
  // Открывает правый щелчок по плитке. ← → — соседние плитки, Enter или «Поставить» — как щелчок по плитке, Esc, ✕, фон — назад.
  // Окно в modalStack: окно оформления под ним молчит (его Esc и фокус), клавиши программы тоже.
  let { sp, scope, hold, dpr }: { sp: SpreadView; scope: BackdropScope; hold: number; dpr: number } = $props()

  const kind = $derived(decor.cur?.kind ?? 'bg')
  const byId = $derived(new Map((decor.cat?.items ?? []).map((i) => [i.id, i])))
  const it = $derived(peek.id ? byId.get(peek.id) : undefined)
  const list = $derived(peek.ids.filter((id) => byId.has(id)))
  const replace = $derived(!!decor.replacing())
  const spread = $derived(kind === 'spread')
  // 0.28-133-41: «Показать весь набор» — у предмета набора; у встроенных и своих набора нет (встроенные — «Набор ▾ — Встроенные»)
  const setId = $derived(it?.pack && it.packId && it.packId !== BUILTIN_PACK ? it.packId : '')
  const checker = $derived(!!it && (kind === 'sticker' || (kind === 'frame' && it.own)))

  let box = $state<HTMLElement>()
  let back = $state<HTMLElement>()
  let me = $state(0)
  let nat = $state({ w: 0, h: 0 }) // размер крупной картинки, px
  let ready = $state(false) // крупная загрузилась: миниатюра под ней убирается (у стикера иначе просвечивает)
  let failed = $state(false) // крупная не загрузилась: остаётся миниатюра, лупы нет, без тоста
  const span = () => Math.max(window.innerWidth, window.innerHeight) // большая сторона окна просмотра: от неё px встроенных

  const thumb = $derived(it ? thumbSrc(it, kind, sp, scope, hold, dpr) : '')
  const big = $derived(it ? peekSrc(it, kind, sp, scope, span(), dpr) : '')
  const guess = $derived({ w: (it ? peekGuess(it, kind, sp, scope) : 1) * 1000, h: 1000 })
  const zoomOn = $derived(!spread && !failed)
  const loupe = new PreviewLoupe(() => ({ size: nat, guess, off: !zoomOn, max: 1 }))
  const at = $derived(loupe.at)
  const mod = isMac() ? '⌘' : 'Ctrl'
  const hint = $derived(keymap.text(peekHint(mod, zoomOn, replace)))

  // Прежняя плитка размер и сбой не передаёт дальше.
  $effect.pre(() => {
    if (peek.id) {
      nat = { w: 0, h: 0 }
      ready = false
      failed = false
    }
  })
  // Загрузился кадр: зум и точка переносятся при тех же пропорциях, иначе вписано.
  $effect(() => {
    if (nat.w > 0 && nat.h > 0) untrack(() => loupe.loaded(nat))
  })
  // Просмотр закрыт — лупа сбрасывается: следующий начинается с «вписано».
  $effect(() => {
    if (!peek.id) untrack(() => loupe.reset())
  })
  // Предмет пропал из каталога (обновили набор, ✎, ×): закрываем, ставить нечего.
  $effect(() => {
    if (peek.id && !it) peek.close()
  })
  // Закрыли окно оформления — просмотр уходит вместе с ним, запись из стека снимает очистка эффекта ниже.
  $effect(() => () => peek.close())
  $effect(() => {
    if (box) box.focus()
  })
  $effect(() => (back ? loupe.bind(back) : undefined))
  const open = $derived(!!it)
  $effect(() => {
    if (!open) return
    const id = untrack(() => modalStack.push(true))
    me = id
    return () => {
      modalStack.remove(id)
    }
  })

  // Назад к плитке: она в виду и в фокусе (после tick: окно оформления, снова верхнее, сначала берёт фокус себе).
  async function leave() {
    const id = peek.id
    peek.close()
    await tick()
    const el = Array.from(document.querySelectorAll<HTMLElement>('[data-decor-id]')).find((e) => e.dataset.decorId === id)
    el?.scrollIntoView?.({ block: 'nearest' })
    el?.focus()
  }
  // Весь набор: окно оформления встаёт на «Все» с отбором по этому набору, просмотр закрывается, плитка в фокусе
  function showSet() {
    if (!setId) return
    decor.showPack(setId)
    void leave()
  }
  // Поставить: как щелчок по плитке (вопросы 067, 077, 115 прежние). Идёт загрузка своих — не ставим.
  function put() {
    const target = it // после close() производное it уже пусто
    if (!target || decor.busy) return
    peek.close()
    void decor.pick(target)
  }

  const OWN = ['photoPreview', 'fitZoom', 'zoomIn', 'zoomOut']
  // Команда лупы с клавиатуры: «Крупно» — 1:1 (автоповтор не мигает), «Вписать», масштаб шагом холста.
  function loupeKey(cmd: string, repeat: boolean) {
    if (cmd === 'photoPreview') {
      if (!repeat) loupe.toggleAt()
    } else if (cmd === 'fitZoom') loupe.fit()
    else loupe.zoomBy(cmd === 'zoomIn' ? ZOOM_STEP : 1 / ZOOM_STEP)
  }
  const arrow = (e: KeyboardEvent) =>
    (e.key === 'ArrowLeft' || e.key === 'ArrowRight') && !e.ctrlKey && !e.metaKey && !e.altKey && !e.shiftKey
  // Своя клавиша окна: Esc, ← →, Enter и команды лупы; остальное идёт диспетчеру (отмена). Возвращает действие или null.
  function action(e: KeyboardEvent): (() => void) | null {
    if (e.key === 'Escape') return () => void leave()
    if (arrow(e)) return () => peek.step(e.key === 'ArrowRight' ? 1 : -1, (id) => byId.has(id))
    // На «Поставить» и ✕ Enter — их собственный щелчок; автоповтор Enter (с пункта меню, из поля) не ставит и не закрывает окно
    if (e.key === 'Enter') return e.target instanceof HTMLButtonElement ? null : () => !e.repeat && put()
    if (!zoomOn) return null
    const chord = eventChord(e)
    const cmd = chord ? OWN.find((c) => keymap.keys(c).includes(chord)) : undefined
    return cmd ? () => loupeKey(cmd, e.repeat) : null
  }
  function onKey(e: KeyboardEvent) {
    if (!it) return
    // Фокус ушёл из окна (Shift+Tab к плитке под ним): клавиша не доходит до плитки, фокус возвращается, как у Modal
    if (box && !box.contains(document.activeElement)) {
      e.preventDefault()
      e.stopPropagation()
      box.focus()
      return
    }
    const run = action(e)
    if (!run) return
    e.preventDefault()
    e.stopPropagation()
    run()
  }

  // Правый щелчок в окне просмотра: закрывает его здесь же. Закрытие по pointerdown правой кнопкой роняло бы
  // contextmenu, что приходит после отпускания, на плитку под курсором, и просмотр открылся бы заново на другой.
  function onMenu(e: MouseEvent) {
    e.preventDefault()
    void leave()
  }

  // Нажатие в окне: по кадру — жест лупы, по пустому месту вокруг — назад.
  function stageDown(e: PointerEvent) {
    if (primaryPress(e) && e.target === e.currentTarget) void leave()
    else if ((e.target as Element).closest('.frame')) loupe.down(e)
  }
  const loaded = (e: Event) => {
    const img = e.currentTarget as HTMLImageElement
    nat = { w: img.naturalWidth, h: img.naturalHeight }
    ready = true
  }
</script>

<svelte:window onkeydowncapture={onKey} />

{#if it}
  <div
    class="back"
    style:z-index={Z_MODAL + Math.max(modalStack.depth(me), 0)}
    role="presentation"
    bind:this={back}
    onpointerdown={(e) => primaryPress(e) && e.target === e.currentTarget && leave()}
    oncontextmenu={onMenu}
    data-chrome
  >
    <div class="box" role="dialog" aria-modal="true" aria-label="Крупный просмотр" tabindex="-1" bind:this={box}>
      <div
        class="view"
        role="presentation"
        bind:this={loupe.stage}
        bind:clientWidth={loupe.box.w}
        bind:clientHeight={loupe.box.h}
        onpointerdown={stageDown}
        onpointermove={(e) => loupe.move(e)}
        onpointerup={(e) => loupe.up(e)}
        onpointercancel={() => loupe.cancel()}
      >
        <!-- Рамка лупы: размер и место из loupe.ts (width/height и сдвиг, а не scale — браузер пересчитывает картинку чётко) -->
        <div
          class="frame"
          style:background={it.color || undefined}
          class:chk={checker && !it.dark}
          class:chkdark={checker && it.dark}
          class:canzoom={zoomOn && loupe.canZoom && !loupe.zoomed}
          class:zoomed={loupe.zoomed}
          class:panning={loupe.panning}
          style:width="{at.w}px"
          style:height="{at.h}px"
          style:transform="translate({at.left}px, {at.top}px)"
        >
          {#if !ready}<img class="ph" src={thumb} alt="" draggable="false" />{/if}
          {#if !failed}
            {#key it.id}
              <img
                class="ph"
                class:wait={!ready}
                src={big}
                alt={it.name}
                draggable="false"
                onload={loaded}
                onerror={() => (failed = true)}
                {@attach cancelOnUnmount}
              />
            {/key}
          {/if}
        </div>
      </div>
      <div class="cap">
        <span class="name">{peekCaption(it, list.indexOf(it.id) + 1, list.length)}</span>
        {#if setId}
          <button class="ghost" title="Показать весь набор «{it.pack}»" onclick={showSet}>Показать весь набор</button>
        {/if}
        <button class="primary" disabled={decor.busy} onclick={put}>{replace ? 'Заменить' : 'Поставить'}</button>
        <button class="x" onclick={leave} title="Закрыть (Esc)" aria-label="Закрыть"><Icon name="x" size={14} /></button>
      </div>
      <div class="hint">{hint}</div>
    </div>
  </div>
{/if}

<style>
  .back {
    position: fixed;
    inset: 0;
    display: grid;
    place-items: center;
    background: rgba(0, 0, 0, 0.82);
  }
  .box {
    position: relative;
    display: grid;
    grid-template-rows: 1fr 36px auto;
    width: 94vw;
    height: 94vh;
    outline: none;
  }
  .view {
    position: relative;
    min-height: 0;
    overflow: hidden;
    touch-action: none;
  }
  /* Подложка как у плитки: встроенный фон и рамочка рисуются на прозрачном холсте (it.color — цвет страниц пресета) */
  .frame {
    position: absolute;
    left: 0;
    top: 0;
    background: var(--panel2);
  }
  .frame.canzoom {
    cursor: zoom-in;
  }
  .frame.zoomed {
    cursor: grab;
  }
  .frame.panning {
    cursor: grabbing;
  }
  /* Прозрачность стикера видна на шашечке; светлый стикер — на тёмной (как плитка) */
  .frame.chk {
    background: repeating-conic-gradient(#d4d4d8 0 25%, #f4f4f5 0 50%) 0 0 / 14px 14px;
  }
  .frame.chkdark {
    background: repeating-conic-gradient(var(--line) 0 25%, var(--panel2) 0 50%) 0 0 / 14px 14px;
  }
  .ph {
    position: absolute;
    inset: 0;
    display: block;
    width: 100%;
    height: 100%;
    object-fit: contain;
    user-select: none;
  }
  /* Крупная ещё грузится: невидима, миниатюра на месте */
  .ph.wait {
    visibility: hidden;
  }
  .cap {
    display: flex;
    align-items: center;
    gap: 12px;
    height: 36px;
    color: var(--muted);
    font-size: 13px;
  }
  .cap .name {
    flex: 1 1 auto;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .hint {
    color: var(--muted);
    font-size: 12px;
    padding-bottom: 2px;
  }
  .x {
    flex: none;
    width: 26px;
    height: 26px;
    padding: 0;
    display: grid;
    place-items: center;
    background: transparent;
    color: var(--text);
  }
</style>
