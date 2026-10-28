<script lang="ts">
  import { untrack } from 'svelte'
  import { store } from '../../base/core/store.svelte'
  import { prefs } from '../../base/prefs/prefs.svelte'
  import { eventChord } from '../../base/keys/chord'
  import { keymap } from '../../base/keys/keymap.svelte'
  import { modalStack, Z_MODAL } from '../../base/modal/modalstack.svelte'
  import { isMac } from '../../base/core/platform'
  import { ZOOM_STEP } from '../../editor/zoom/zoom'
  import { ORIG_LOADING } from '../../editor/photo/loupe'
  import Icon from '../../base/ui/Icon.svelte'
  import CompareTile from './CompareTile.svelte'
  import { compare } from './compare.svelte'
  import { compareHint, ownKey, title } from './compareset'

  // Сравнение 2–4 кадров рядом (138). Открывают C, Shift+C и меню ленты. Образец — окно 128 (PhotoPreview): тот же фон,
  // подпись, значок зума и строка клавиш; окно в modalStack с undo — под ним работают отмена и отметки активному кадру
  // (их пускает диспетчер, shell/menu/keys.ts), остальные клавиши программы молчат.
  const lp = compare.loupe
  const open = $derived(compare.open)
  let box = $state<HTMLElement>()
  let back = $state<HTMLElement>()
  let tilesEl = $state<HTMLElement>()

  // Окно — в стек, пока оно открыто (любой путь закрытия освобождает его).
  let me = $state(0)
  const top = $derived(me > 0 && modalStack.isTop(me))
  $effect(() => {
    if (!open) return
    const mid = untrack(() => modalStack.push(true))
    me = mid
    return () => {
      modalStack.remove(mid)
      me = 0
    }
  })
  // Фокус — на активной ячейке (Tab перебирает только ячейки); вопрос сверху закрылся — возвращается сюда.
  $effect(() => {
    const a = compare.activeId
    if (box && top && a) box.querySelector<HTMLElement>(`[data-i="${compare.active}"]`)?.focus({ preventScroll: true })
  })
  // Кадр пропал из альбома (правка за окном) — ячейка уходит; осталась одна — окно закрывается.
  $effect(() => {
    void store.byId
    untrack(() => compare.prune())
  })
  // Колесо и щипок Mac (134): passive: false, иначе Ctrl+колесо масштабирует страницу.
  $effect(() => (back ? lp.bind(back) : undefined))

  const mod = isMac() ? '⌘' : 'Ctrl'
  const hint = $derived(keymap.text(compareHint(mod, prefs.can('marks'))))
  const act = $derived(compare.activeId)
  // Пока оригинал активной ячейки не пришёл — «Оригинал загружается…»; на пределе без оригинала — причина рядом (Т9).
  const label = $derived(act && lp.scale(compare.active) > 1 + 1e-6 && compare.orig(act).waiting(act) ? ORIG_LOADING : lp.label)
  const note = $derived(compare.note)

  // Свои клавиши окна: Esc, ← →, Tab, Enter; из реестра — «Сравнить» (закрыть), «Убрать из подборки» (Delete: убрать кадр), а также «Крупно» (1:1 у всех),
  // «Вписать» и масштаб — они не дойдут до холста под окном. Остальное — диспетчеру: отмена и отметки.
  const OWN = ['photoPreview', 'fitZoom', 'zoomIn', 'zoomOut', 'photoCompare', 'binRemove']
  function onKey(e: KeyboardEvent) {
    if (!compare.open || !top) return
    const own = () => {
      e.preventDefault()
      e.stopPropagation()
    }
    const k = ownKey(e, e.target instanceof HTMLButtonElement)
    if (k) {
      own()
      if (k === 'close') compare.close()
      else if (k === 'prev' || k === 'next') compare.step(k === 'next' ? 1 : -1)
      else if (k === 'tab' || k === 'shiftTab') compare.cycle(k === 'tab' ? 1 : -1)
      else void compare.best()
      return
    }
    const chord = eventChord(e)
    const cmd = chord ? OWN.find((c) => keymap.keys(c).includes(chord)) : undefined
    if (!cmd) return
    own()
    loupeKey(cmd, e.repeat)
  }
  // Команда лупы с клавиатуры: «Сравнить» закрывает, «Крупно» — 1:1 у всех (автоповтор не мигает), «Вписать», масштаб шагом холста.
  function loupeKey(cmd: string, repeat: boolean) {
    if (cmd === 'photoCompare') compare.close()
    else if (cmd === 'binRemove') compare.remove()
    else if (cmd === 'photoPreview') {
      if (!repeat) lp.toggleAt()
    } else if (cmd === 'fitZoom') lp.fit()
    else lp.zoomBy(cmd === 'zoomIn' ? ZOOM_STEP : 1 / ZOOM_STEP)
  }

  // Нажатие мимо ячеек (фон, рамка окна, пустое место между кадрами) закрывает окно.
  function bgDown(e: PointerEvent) {
    if (e.target === back || e.target === box || e.target === tilesEl) compare.close()
  }
</script>

<svelte:window onkeydowncapture={onKey} />

{#if open}
  <div
    class="back"
    role="presentation"
    style:z-index={Z_MODAL + Math.max(modalStack.depth(me), 0)}
    bind:this={back}
    onpointerdown={bgDown}
    data-chrome
  >
    <div class="box" role="dialog" aria-modal="true" aria-label="Сравнение кадров" tabindex="-1" bind:this={box}>
      <div class="tiles" role="presentation" bind:this={tilesEl} bind:clientWidth={compare.box.w} bind:clientHeight={compare.box.h}>
        {#each compare.ids as id, i (id)}
          <CompareTile {i} {id} />
        {/each}
      </div>
      <div class="cap">
        <span class="name">{title(compare.ids.length)}</span>
        {#if note}<span class="note">{note}</span>{/if}
        {#if label}<span class="zoom" class:after={!!note} title={note || undefined}>{label}</span>{/if}
        <button class="x" onclick={() => compare.close()} title="Закрыть (Esc)" aria-label="Закрыть"><Icon name="x" size={14} /></button>
      </div>
      <div class="hint">{hint}</div>
    </div>
  </div>
{/if}

<style>
  .back {
    position: fixed;
    inset: 0;
    z-index: 85; /* запас: реальный z — из modalStack.depth (style:z-index) */
    display: grid;
    place-items: center;
    background: rgba(0, 0, 0, 0.82);
  }
  .box {
    position: relative;
    display: grid;
    grid-template-rows: minmax(0, 1fr) 36px auto;
    width: 96vw;
    height: 96vh;
    outline: none;
  }
  .tiles {
    position: relative;
    min-height: 0;
    overflow: hidden;
    touch-action: none;
  }
  .cap {
    display: flex;
    align-items: center;
    gap: 12px;
    height: 36px;
    color: var(--muted);
    font-size: 13px;
  }
  .name {
    flex: 0 1 auto;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: var(--text);
  }
  .note {
    margin-left: auto;
    color: var(--muted);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    min-width: 0;
  }
  .zoom {
    margin-left: auto;
    padding: 2px 8px;
    border-radius: 4px;
    background: var(--line);
    color: var(--text);
    white-space: nowrap;
  }
  .zoom.after {
    margin-left: 0;
  }
  /* Строка длиннее, чем в 128 (214 знаков): 11 px, чтобы при 1280 помещалась в одну строку; в узком окне переносится под кадрами */
  .hint {
    color: var(--muted);
    font-size: 11px;
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
  /* Без значка зума и заметки кнопка закрытия всё равно прижата вправо */
  .name + .x {
    margin-left: auto;
  }
</style>
