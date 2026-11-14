<script lang="ts">
  import { tick, untrack, type Snippet } from 'svelte'
  import { primaryPress } from '../../base/core/platform'
  import { modalStack, Z_MODAL } from '../../base/modal/modalstack.svelte'
  import Icon from '../../base/ui/Icon.svelte'

  // Оболочка крупного просмотра окон-галерей (133 U2, 163 U1): подложка, коробка 94×94 %, запись в modalStack, Esc, правый щелчок
  // и щелчок по фону — назад, фокус возвращается плитке. Что внутри — решает вызывающий (DecorPeek — лупа, LookPeek — лист шага):
  // view — кадр, strip — необязательная лента под ним, caption — подпись, actions — кнопки перед ✕, hint — строка клавиш.
  // Монтируется, пока просмотр открыт: окно в стеке ровно столько, сколько живёт оболочка.
  let {
    tileId,
    onclose,
    onkey,
    tileSel = '[data-decor-id]',
    label = 'Крупный просмотр',
    back = $bindable(),
    view,
    strip,
    caption,
    actions,
    hint,
  }: {
    tileId: string // плитка, которой вернётся фокус (значение её data-decor-id)
    onclose: () => void // закрыть просмотр (состояние у вызывающего)
    onkey?: (e: KeyboardEvent) => (() => void) | null // своя клавиша окна (кроме Esc): действие или null
    tileSel?: string // где искать плитки для фокуса назад
    label?: string
    back?: HTMLElement // подложка: лупа вешает на неё колесо и щипок
    view: Snippet<[() => Promise<void>]> // получает leave: щелчок по пустому месту вокруг кадра — назад
    strip?: Snippet
    caption: Snippet
    actions?: Snippet<[() => Promise<void>]> // получает leave: «Показать весь набор» закрывает просмотр так же, как ✕
    hint: Snippet
  } = $props()

  let box = $state<HTMLElement>()
  let me = $state(0)

  $effect(() => {
    const id = untrack(() => modalStack.push(true))
    me = id
    return () => modalStack.remove(id)
  })
  $effect(() => {
    if (box) box.focus()
  })

  // Назад к плитке: она в виду и в фокусе (после tick: окно под просмотром, снова верхнее, сначала берёт фокус себе).
  async function leave() {
    const id = tileId
    const sel = tileSel
    onclose()
    await tick()
    const el = Array.from(document.querySelectorAll<HTMLElement>(sel)).find((e) => e.dataset.decorId === id)
    el?.scrollIntoView?.({ block: 'nearest' })
    el?.focus()
  }

  // ← → и Enter в поле ввода, списке, переключателе — их собственные: окно их не берёт (163-20)
  const FIELD = 'input, select, textarea, [role="radio"]'
  const own = (e: KeyboardEvent) =>
    (e.key === 'ArrowLeft' || e.key === 'ArrowRight' || e.key === 'Enter') &&
    e.target instanceof Element &&
    e.target.closest(FIELD) !== null

  function onKey(e: KeyboardEvent) {
    // Поверх просмотра вопрос или другое окно: клавиши и фокус их, не наши (163-20)
    if (!modalStack.isTop(me)) return
    // Фокус ушёл из окна (Shift+Tab к плитке под ним): клавиша не доходит до плитки, фокус возвращается, как у Modal
    if (box && !box.contains(document.activeElement)) {
      e.preventDefault()
      e.stopPropagation()
      box.focus()
      return
    }
    if (own(e)) return
    const run = e.key === 'Escape' ? () => void leave() : (onkey?.(e) ?? null)
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
</script>

<svelte:window onkeydowncapture={onKey} />

<div
  class="back"
  style:z-index={Z_MODAL + Math.max(modalStack.depth(me), 0)}
  role="presentation"
  bind:this={back}
  onpointerdown={(e) => primaryPress(e) && e.target === e.currentTarget && leave()}
  oncontextmenu={onMenu}
  data-chrome
>
  <div
    class="box"
    style:grid-template-rows={strip ? '1fr auto 36px auto' : '1fr 36px auto'}
    role="dialog"
    aria-modal="true"
    aria-label={label}
    tabindex="-1"
    bind:this={box}
  >
    {@render view(leave)}
    {@render strip?.()}
    <div class="cap">
      <span class="name">{@render caption()}</span>
      {@render actions?.(leave)}
      <button class="x" onclick={leave} title="Закрыть (Esc)" aria-label="Закрыть"><Icon name="x" size={14} /></button>
    </div>
    <div class="hint">{@render hint()}</div>
  </div>
</div>

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
    width: 94vw;
    height: 94vh;
    outline: none;
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
