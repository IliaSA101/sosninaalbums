<script lang="ts">
  import { hexError, loadRecent, normalizeHex, rememberColor, SWATCHES } from './palette'
  import { placePopover, type Placed } from './popover'

  // Своя палитра вместо системного выбора цвета: образец-кнопка и окно в пределах окна программы.
  // onpick зовётся один раз на выбор (образец, «последний», HEX по Enter / уходу из поля, пипетка) —
  // родитель делает из него одну правку, то есть один шаг Ctrl+Z.
  let {
    value,
    onpick,
    label = 'Цвет',
    disabled = false,
  }: { value: string; onpick: (hex: string) => void; label?: string; disabled?: boolean } = $props()

  // EyeDropper есть не во всех WebView2 — без него кнопки нет.
  interface EyeDropperLike {
    open(): Promise<{ sRGBHex: string }>
  }
  const Eye = (globalThis as { EyeDropper?: new () => EyeDropperLike }).EyeDropper

  let open = $state(false)
  let btn = $state<HTMLButtonElement>()
  let pop = $state<HTMLElement>()
  let placed = $state<Placed | null>(null)
  let recent = $state<string[]>([])
  let draft = $state('')
  let error = $state('')
  let picking = false // пипетка активна: щелчок по экрану не должен закрывать окно
  let focused = false // фокус в окно — один раз на открытие, а не при каждом пересчёте положения

  // Окно уходит в body: внутри Inspector (свой stacking context) его перекрывала бы лента альбома.
  function portal(node: HTMLElement) {
    document.body.appendChild(node)
    return { destroy: () => node.remove() }
  }

  const cur = $derived(normalizeHex(value) ?? '')

  function show() {
    recent = loadRecent()
    draft = cur
    error = ''
    placed = null
    focused = false
    open = true
  }

  function close() {
    open = false
    btn?.focus()
  }

  // Положение считаем, когда окно уже отрисовано и известен его размер; сообщение об ошибке HEX
  // его увеличивает — пересчитываем (переворот вверх / подъём), чтобы низ не обрезался.
  $effect(() => {
    if (!open || !pop || !btn) return
    void error
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

  function pick(hex: string) {
    const h = normalizeHex(hex)
    if (!h) return
    rememberColor(h)
    open = false
    onpick(h)
  }

  function applyDraft() {
    if (!open) return
    const e = hexError(draft)
    if (e) {
      error = draft.trim() === '' ? '' : e
      return
    }
    error = ''
    pick(draft)
  }

  async function eyedrop() {
    if (!Eye) return
    picking = true
    try {
      const r = await new Eye().open()
      pick(r.sRGBHex)
    } catch {
      // Отмена пипетки (Esc) — не ошибка.
    } finally {
      picking = false
    }
  }

  function onKeyCapture(e: KeyboardEvent) {
    if (!open || e.key !== 'Escape') return
    // Esc закрывает только палитру: выделение и галерею под ней не трогаем.
    e.stopPropagation()
    e.preventDefault()
    close()
  }

  function onPointerCapture(e: PointerEvent) {
    if (!open || picking) return
    const t = e.target as Node
    if (!pop?.contains(t) && !btn?.contains(t)) open = false
  }
</script>

<!-- Окно вне панели не едет за её прокруткой — при прокрутке закрываем -->
<svelte:window
  onkeydowncapture={onKeyCapture}
  onpointerdowncapture={onPointerCapture}
  onscrollcapture={(e) => open && !pop?.contains(e.target as Node) && (open = false)}
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
    style:left="{placed?.left ?? 0}px"
    style:top="{placed?.top ?? 0}px"
    style:visibility={placed ? 'visible' : 'hidden'}
  >
    <div class="grid">
      {#each SWATCHES as c (c)}
        <button type="button" class="chip" class:on={c === cur} style:--c={c} title={c} aria-label={c} onclick={() => pick(c)}></button>
      {/each}
    </div>

    <div class="cap">Последние</div>
    <div class="grid">
      {#each recent as c (c)}
        <button type="button" class="chip" class:on={c === cur} style:--c={c} title={c} aria-label={c} onclick={() => pick(c)}></button>
      {:else}
        <span class="none">пока нет</span>
      {/each}
    </div>

    <div class="hex">
      <input
        type="text"
        spellcheck="false"
        maxlength="7"
        aria-label="Цвет в формате HEX"
        aria-invalid={error ? 'true' : undefined}
        placeholder="#rrggbb"
        bind:value={draft}
        oninput={() => (error = '')}
        onkeydown={(e) => e.key === 'Enter' && (e.preventDefault(), applyDraft())}
        onblur={(e) => !pop?.contains(e.relatedTarget as Node | null) && applyDraft()}
      />
      {#if Eye}
        <button type="button" class="eye" title="Взять цвет с экрана" onclick={eyedrop}>Пипетка</button>
      {/if}
    </div>
    {#if error}<div class="err" role="alert">{error}</div>{/if}
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
    width: 188px;
    padding: 10px;
    background: var(--panel);
    border: 1px solid var(--line);
    border-radius: 8px;
    box-shadow: 0 10px 32px rgba(0, 0, 0, 0.5);
    font-size: 12px;
    outline: none;
  }
  .grid {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
    min-height: 20px;
  }
  .chip {
    width: 20px;
    height: 20px;
    padding: 0;
    border-radius: 4px;
    border: 1px solid var(--line);
    background: var(--c);
  }
  .chip:hover:not(:disabled) {
    background: var(--c);
    border-color: var(--text);
  }
  .chip.on {
    outline: 2px solid var(--accent);
    outline-offset: 1px;
  }
  .cap {
    margin: 10px 0 6px;
    color: var(--muted);
  }
  .none {
    color: var(--muted);
    line-height: 20px;
  }
  .hex {
    display: flex;
    gap: 6px;
    margin-top: 10px;
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
  .hex input[aria-invalid='true'] {
    border-color: #d9534f;
  }
  .eye {
    height: 28px;
    padding: 0 8px;
  }
  .err {
    margin-top: 6px;
    color: #ff8a80;
  }
</style>
