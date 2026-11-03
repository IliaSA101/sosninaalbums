<script lang="ts">
  import { untrack } from 'svelte'
  import type { BackdropScope, SpreadView } from '../../base/core/types'
  import { eventChord } from '../../base/keys/chord'
  import { keymap } from '../../base/keys/keymap.svelte'
  import { isMac, primaryPress } from '../../base/core/platform'
  import { cancelOnUnmount } from '../../base/ui/imgcancel'
  import { PreviewLoupe } from '../../editor/photo/loupe.svelte'
  import { ZOOM_STEP } from '../../editor/zoom/zoom'
  import { decor } from '../decor/decor.svelte'
  import { peek } from './peek.svelte'
  import PeekShell from './PeekShell.svelte'
  import { BUILTIN_PACK } from './browse'
  import { peekCaption, peekGuess, peekHint, peekSrc, thumbSrc } from './peekaddr'

  // Крупный просмотр плитки окон оформления (133 U2), образец — PhotoPreview: вписано, шашечка, лупа, подпись, ✕.
  // Открывает правый щелчок по плитке. ← → — соседние плитки, Enter или «Поставить» — как щелчок по плитке, Esc, ✕, фон — назад.
  // Оболочка (PeekShell, общая с окном макетов) держит окно в modalStack: окно оформления под ним молчит, клавиши программы тоже.
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

  let back = $state<HTMLElement>()
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
  const hintText = $derived(keymap.text(peekHint(mod, zoomOn, replace)))

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
  // Закрыли окно оформления — просмотр уходит вместе с ним.
  $effect(() => () => peek.close())
  $effect(() => (back ? loupe.bind(back) : undefined))

  // Весь набор: окно оформления встаёт на «Все» с отбором по этому набору, просмотр закрывается, плитка в фокусе
  function showSet(leave: () => Promise<void>) {
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
  // Своя клавиша окна: ← →, Enter и команды лупы (Esc и поля ввода — в PeekShell); остальное идёт диспетчеру (отмена).
  // Возвращает действие или null.
  function action(e: KeyboardEvent): (() => void) | null {
    if (arrow(e)) return () => peek.step(e.key === 'ArrowRight' ? 1 : -1, (id) => byId.has(id))
    // На «Поставить» и ✕ Enter — их собственный щелчок; автоповтор Enter (с пункта меню, из поля) не ставит и не закрывает окно
    if (e.key === 'Enter') return e.target instanceof HTMLButtonElement ? null : () => !e.repeat && put()
    if (!zoomOn) return null
    const chord = eventChord(e)
    const cmd = chord ? OWN.find((c) => keymap.keys(c).includes(chord)) : undefined
    return cmd ? () => loupeKey(cmd, e.repeat) : null
  }
  // Нажатие в окне: по кадру — жест лупы, по пустому месту вокруг — назад.
  function stageDown(e: PointerEvent, leave: () => Promise<void>) {
    if (primaryPress(e) && e.target === e.currentTarget) void leave()
    else if ((e.target as Element).closest('.frame')) loupe.down(e)
  }
  const loaded = (e: Event) => {
    const img = e.currentTarget as HTMLImageElement
    nat = { w: img.naturalWidth, h: img.naturalHeight }
    ready = true
  }
</script>

{#if it}
  <PeekShell tileId={it.id} onclose={() => peek.close()} onkey={action} bind:back>
    {#snippet view(leave)}
      <div
        class="view"
        role="presentation"
        bind:this={loupe.stage}
        bind:clientWidth={loupe.box.w}
        bind:clientHeight={loupe.box.h}
        onpointerdown={(e) => stageDown(e, leave)}
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
    {/snippet}
    {#snippet caption()}{peekCaption(it, list.indexOf(it.id) + 1, list.length)}{/snippet}
    {#snippet actions(leave)}
      {#if setId}
        <button class="ghost" title="Показать весь набор «{it.pack}»" onclick={() => showSet(leave)}>Показать весь набор</button>
      {/if}
      <button class="primary" disabled={decor.busy} onclick={put}>{replace ? 'Заменить' : 'Поставить'}</button>
    {/snippet}
    {#snippet hint()}{hintText}{/snippet}
  </PeekShell>
{/if}

<style>
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
</style>
