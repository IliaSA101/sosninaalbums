<script lang="ts">
  import { portal } from '../ui/portal'
  import { onDestroy } from 'svelte'
  import { store } from '../core/store.svelte'
  import Icon from '../ui/Icon.svelte'
  import { placePopover, type Placed } from '../ui/popover'
  import ColorSets, { type PhotoColors } from './ColorSets.svelte'
  import HsvField from './HsvField.svelte'
  import MyColors from './MyColors.svelte'
  import { colorWords, eyeHint } from './colorwords'
  import { eyedrop } from './eyedrop.svelte'
  import { mycolors } from './mycolors.svelte'
  import { HEX_PREFIX, hexError, loadTab, normalizeHex, PHOTO_TAB, saveTab, SETS } from './palette'

  // Своя палитра вместо системного выбора цвета (109, вариант 1): образец-кнопка и окно в пределах окна программы.
  // onpick зовётся один раз на выбор (образец, «Свои», HEX по Enter / уходу из поля, пипетка, отпускание поля оттенка) —
  // родитель делает из него одну правку, то есть один шаг Ctrl+Z. Вернул промис — показ держится до ответа Go.
  // onpreview(hex) — живой показ на развороте (наведение, фокус, протяжка по полю, правильный HEX в поле), onpreview(null) —
  // снять: уход с образца, закрытие без выбора, вход в режим пипетки, ответ Go после выбора. В историю и файл показ не
  // попадает (109-6). sheet — лист для вкладки «С фото» и пипетки; без него их нет.
  let {
    value,
    onpick,
    onpreview,
    sheet,
    label = 'Цвет',
    disabled = false,
  }: {
    value: string
    onpick: (hex: string) => unknown
    onpreview?: (hex: string | null) => void
    sheet?: string
    label?: string
    disabled?: boolean
  } = $props()

  const me = {} // владелец режима пипетки: окно, которое его запустило

  let open = $state(false)
  let btn = $state<HTMLButtonElement>()
  let pop = $state<HTMLElement>()
  let placed = $state<Placed | null>(null)
  let draft = $state('')
  let error = $state('')
  let tab = $state(SETS[0].id)
  let hsvOpen = $state(false)
  let photo = $state<PhotoColors>({ state: 'load', list: [] })
  let shown = $state<string | null>(null) // цвет под мышью / в поле: шапка окна показывает его
  let focused = false // фокус в окно — один раз на открытие, а не при каждом пересчёте положения
  let previewing = false // родителю послан показ, который ещё не снят
  let picked = false // выбор сделан, правка в пути: уход мыши с образца показ не снимает
  let live = false // пока правка в пути, пошёл новый показ (вторая протяжка): ответ Go его не снимает

  const cur = $derived(normalizeHex(value) ?? '')
  // Ошибка HEX: после Enter / ухода из поля; а то, что уже не может стать цветом («zzz»), — сразу при вводе.
  const hexMsg = $derived(error || (draft.trim() !== '' && !HEX_PREFIX.test(draft.trim()) ? (hexError(draft) ?? '') : ''))

  const words = $derived(colorWords(store.view.settings, sheet)) // лист / разворот / обложка в подписях
  const eye = $derived(eyedrop.owner === me) // режим «Взять цвет с фото» идёт, и это наше окно

  // Показ родителю; снятие без показа — пустой вызов не нужен.
  function preview(hex: string | null) {
    if (hex === null ? picked || !previewing : picked && !open) return
    if (picked && hex !== null) live = true
    previewing = hex !== null
    shown = hex
    onpreview?.(hex)
  }

  function show() {
    void mycolors.load()
    draft = cur
    error = ''
    placed = null
    focused = false
    hsvOpen = false
    // Последняя вкладка (109-8); «С фото» без листа не бывает.
    const t = loadTab()
    tab = t === PHOTO_TAB && !sheet ? SETS[0].id : t
    open = true
  }

  function close() {
    open = false
    btn?.focus()
  }

  function setTab(id: string) {
    tab = id
    saveTab(id)
  }

  // Закрыли без выбора (Esc, щелчок мимо, прокрутка, размер окна): показ снимается, режим пипетки — тоже.
  $effect(() => {
    if (open) return
    if (!picked) preview(null)
    eyedrop.cancel(me)
  })
  onDestroy(() => {
    preview(null)
    eyedrop.cancel(me)
  })

  // «С фото»: цвета листа спрашиваем при открытии вкладки, на каждое открытие заново (правки листа меняют ответ).
  $effect(() => {
    if (!open || tab !== PHOTO_TAB || !sheet) return
    let alive = true
    photo = { state: 'load', list: [] }
    mycolors.photoColors(sheet).then(
      (list) => alive && (photo = { state: 'ok', list }),
      () => alive && (photo = { state: 'err', list: [] }),
    )
    return () => (alive = false)
  })

  // Положение считаем, когда окно уже отрисовано и известен его размер; вкладка, поле оттенка, приход «С фото», «Свои»
  // и ошибка HEX меняют высоту — пересчитываем (переворот вверх / подъём), чтобы низ не обрезался.
  $effect(() => {
    if (!open || !pop || !btn) return
    void [hexMsg, tab, hsvOpen, photo.state, photo.list.length, mycolors.list.length]
    const a = btn.getBoundingClientRect()
    placed = placePopover(
      { left: a.left, top: a.top, width: a.width, height: a.height },
      { width: pop.offsetWidth, height: pop.offsetHeight },
      { width: window.innerWidth, height: window.innerHeight },
    )
    if (!focused) {
      focused = true
      pop.focus()
    }
  })

  // keep — окно остаётся открытым (поле оттенка: можно тянуть дальше, каждое отпускание — своя правка).
  function pick(hex: string, keep = false) {
    const h = normalizeHex(hex)
    if (!h) return
    if (!keep) open = false
    picked = true
    live = false
    draft = h
    // Показ держится, пока Go не ответил: старый цвет не мелькает. Ответ (или отказ) снимает его.
    new Promise((res) => res(onpick(h))).finally(() => {
      picked = false
      if (live) {
        live = false // уже идёт новая протяжка — её показ не трогаем
        return
      }
      previewing = false
      shown = null
      onpreview?.(null)
    })
    void mycolors.remember(h)
  }

  function applyDraft(force: boolean) {
    if (!open) return
    const e = hexError(draft)
    if (e) {
      error = draft.trim() === '' ? '' : e
      return
    }
    error = ''
    // Уход из поля без правки (Tab мимо) окно не закрывает и цвет не выбирает.
    if (force || normalizeHex(draft) !== cur) pick(draft)
  }

  // Уход в другое место окна: цвет не выбираем, но неверный HEX помечаем.
  function markError() {
    const e = hexError(draft)
    error = e && draft.trim() !== '' ? e : ''
  }

  // Режим пипетки: окно остаётся открытым, цвет берёт слой на холсте (U4) и зовёт eyedrop.finish.
  function eyedropper() {
    if (eye || !sheet) return eyedrop.cancel(me)
    preview(null)
    eyedrop.start(
      me,
      sheet,
      (hex) => pick(hex),
      (t) => !!(pop?.contains(t) || btn?.contains(t)),
    )
  }

  async function forget(hex: string) {
    await mycolors.forget(hex)
    pop?.focus() // фокус был на убранном образце
  }

  function onKeyCapture(e: KeyboardEvent) {
    if (!open || e.key !== 'Escape') return
    // Esc закрывает только палитру: выделение и галерею под ней не трогаем. В режиме пипетки — только выход из него.
    e.stopPropagation()
    e.preventDefault()
    if (eye) eyedrop.cancel(me)
    else close()
  }

  // Окно цвета — виджет для клавиш (Delete на образце, стрелки, Backspace): команды программы (удалить выделенное,
  // листать развороты) до них не доходят. Esc ведёт захват выше; Enter и Tab остаются за полем и кнопками.
  function onPopKey(e: KeyboardEvent) {
    if (e.key !== 'Escape') e.stopPropagation()
  }

  function onPointerCapture(e: PointerEvent) {
    if (!open) return
    const t = e.target as Element
    if (pop?.contains(t) || btn?.contains(t)) return
    // Щелчок мимо в режиме пипетки гасит eyedrop.press (раньше всех слушателей); окно остаётся открытым.
    if (eye) return
    open = false
  }
</script>

<!-- Окно вне панели не едет за её прокруткой — при прокрутке закрываем -->
<svelte:window
  onkeydowncapture={onKeyCapture}
  onpointerdowncapture={onPointerCapture}
  onscrollcapture={(e) => open && !eye && !pop?.contains(e.target as Node) && (open = false)}
  onresize={() => (open = false)}
/>

<button
  bind:this={btn}
  class="sw"
  type="button"
  {disabled}
  title={label}
  aria-label={label}
  aria-haspopup="dialog"
  aria-expanded={open}
  style:--c={cur || 'transparent'}
  class:empty={!cur}
  onclick={() => (open ? close() : show())}
></button>

{#if open}
  <div
    bind:this={pop}
    class="pop"
    use:portal
    data-chrome
    data-text-ui
    role="dialog"
    aria-label={label}
    tabindex="-1"
    onkeydown={onPopKey}
    style:left="{placed?.left ?? 0}px"
    style:top="{placed?.top ?? 0}px"
    style:visibility={placed ? 'visible' : 'hidden'}
  >
    <ColorSets
      shown={shown ?? cur}
      {cur}
      {tab}
      withPhoto={!!sheet}
      {photo}
      {words}
      ontab={setTab}
      onpick={(h) => pick(h)}
      onpreview={preview}
    />
    <MyColors
      list={mycolors.list}
      {cur}
      onpick={(h) => pick(h)}
      onpreview={preview}
      onadd={(h) => void mycolors.add(h)}
      onforget={forget}
    />

    {#if sheet}
      <div class="sep"></div>
      <button type="button" class="eye" class:on={eye} aria-pressed={eye} onclick={eyedropper}>
        <Icon name="pipette" size={18} />
        <span class="txt">
          <b>Взять цвет с фото</b>
          <span>{eye ? 'Щёлкните по развороту. Esc — отмена' : eyeHint(words)}</span>
        </span>
      </button>
    {/if}

    <div class="hex">
      <input
        type="text"
        spellcheck="false"
        maxlength="7"
        aria-label="Цвет в формате HEX"
        class:bad={!!hexMsg}
        aria-invalid={hexMsg ? 'true' : undefined}
        placeholder="#rrggbb"
        bind:value={draft}
        oninput={() => {
          error = ''
          const h = normalizeHex(draft)
          if (h) preview(h)
        }}
        onkeydown={(e) => e.key === 'Enter' && (e.preventDefault(), applyDraft(true))}
        onblur={(e) => (pop?.contains(e.relatedTarget as Node | null) ? markError() : applyDraft(false))}
      />
      <button type="button" class="more" aria-expanded={hsvOpen} onclick={() => (hsvOpen = !hsvOpen)}>
        Свой оттенок {hsvOpen ? '▾' : '▸'}
      </button>
    </div>
    {#if hexMsg}<div class="err" role="alert">{hexMsg}</div>{/if}

    {#if hsvOpen}
      <HsvField hex={cur} onpreview={preview} oncommit={(h) => pick(h, true)} oncancel={() => preview(null)} />
    {/if}
  </div>
{/if}

<style>
  .sw {
    width: 30px;
    height: 24px;
    padding: 0;
    border: 1px solid var(--line);
    border-radius: 5px;
    background: var(--c);
  }
  .sw:hover:not(:disabled) {
    background: var(--c);
    border-color: var(--muted);
  }
  /* Цвет не задан: диагональная черта вместо заливки */
  .sw.empty {
    background: linear-gradient(135deg, transparent 46%, var(--muted) 46% 54%, transparent 54%), var(--panel2);
  }
  .pop {
    position: fixed;
    z-index: 90;
    width: 272px;
    box-sizing: border-box;
    padding: 10px;
    background: var(--panel);
    border: 1px solid var(--line);
    border-radius: 8px;
    box-shadow: 0 10px 32px rgba(0, 0, 0, 0.5);
    font-size: 12px;
    outline: none;
  }
  .sep {
    height: 1px;
    margin: 10px 0 8px;
    background: var(--line);
  }
  .eye {
    display: flex;
    align-items: center;
    gap: 8px;
    width: 100%;
    min-height: 34px;
    height: auto;
    padding: 6px 10px;
    text-align: left;
    white-space: normal;
  }
  .eye.on {
    border-color: var(--accent);
  }
  .eye b {
    display: block;
    font-weight: 600;
    font-size: 12px;
  }
  .eye .txt span {
    display: block;
    color: var(--muted);
    font-size: 11px;
    line-height: 1.3;
  }
  .hex {
    display: flex;
    align-items: center;
    gap: 6px;
    margin-top: 8px;
  }
  .hex input {
    flex: 1;
    min-width: 0;
    height: 28px;
    font: inherit;
    font-family: Consolas, monospace;
    background: #3a3a3e;
    color: var(--text);
    border: 1px solid var(--line);
    border-radius: 6px;
    padding: 0 8px;
    user-select: text;
  }
  .hex input.bad {
    border-color: #d9534f;
  }
  .more {
    height: 28px;
    padding: 0 8px;
    font-size: 12px;
    background: transparent;
    color: var(--muted);
  }
  .more:hover {
    background: #3a3a3e;
    color: var(--text);
  }
  .err {
    margin-top: 4px;
    color: #ff8a80;
  }
</style>
