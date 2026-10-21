<script lang="ts">
  import { tick } from 'svelte'
  import { placePopover, type Placed } from '../../base/ui/popover'
  import Icon from '../../base/ui/Icon.svelte'
  import Modal from '../../base/modal/Modal.svelte'
  import { store } from '../../base/core/store.svelte'
  import type { FontFamily, FontUse } from '../../base/core/types'
  import { findByPrefix, fontCss, groupFonts, moveActive, sampleOf } from './fonts'
  import { HIDE_TITLE, LAT_ONLY, MISSING_TIP, UPLOAD_LABEL, hideFont, hideText, shownFont, uploadFonts } from './fontupload'

  // Выбор шрифта (спека 028 Г): кнопка с текущей гарнитурой и список по группам. Строка — «Анна и Павел»
  // самой гарнитурой и имя справа. Образцы рисуются только в открытом списке — файлы шрифтов грузятся
  // лениво, при первом показе. Фокус остаётся там, где был (в редакторе текста): клавиши ловим на window.
  // onpick зовётся один раз на выбор — родитель делает из него один шаг отмены; им же ставится единственный
  // только что загруженный шрифт (082-20). used — свои шрифты альбома (View.albumFonts): имя шрифта, которого
  // нет на этом компьютере.
  let { value, fonts, used, onpick }: { value: string; fonts: FontFamily[]; used?: FontUse[]; onpick: (id: string) => void } = $props()

  let open = $state(false)
  let btn = $state<HTMLButtonElement>()
  let pop = $state<HTMLElement>()
  let placed = $state<Placed | null>(null)
  let active = $state(-1)
  let width = $state(0)
  let typed = ''
  let typedAt = 0

  const groups = $derived(groupFonts(fonts, value))
  const flat = $derived(groups.flatMap((g) => g.fonts))
  const cur = $derived(fonts.find((f) => f.id === value))
  const shown = $derived(shownFont(value, fonts, used))
  // × и вопрос «Убрать из библиотеки?»: окно вне списка — список закрывается по щелчку мимо и унёс бы его с собой.
  let ask = $state<{ id: string; name: string } | null>(null)
  let busy = $state(false)
  const curFamily = $derived(cur ? fontCss({ font: cur.id }, fonts).family : 'inherit')
  // Номер строки в общем списке по группам: группы идут подряд.
  const offsets = $derived(groups.map((_, i) => groups.slice(0, i).reduce((n, g) => n + g.fonts.length, 0)))
  const css = (f: FontFamily) => fontCss({ font: f.id }, fonts).family

  // Окно уходит в body: у карточки свой контекст наложения, лента альбома его перекрыла бы.
  function portal(node: HTMLElement) {
    document.body.appendChild(node)
    return { destroy: () => node.remove() }
  }

  function show() {
    active = Math.max(
      0,
      flat.findIndex((f) => f.id === value),
    )
    width = btn?.offsetWidth ?? 0
    placed = null
    typed = ''
    open = true
  }

  function close() {
    open = false
  }

  function pick(i: number) {
    const f = flat[i]
    open = false
    if (f && f.id !== value) onpick(f.id)
  }

  // «Загрузить свой шрифт…»: список закрывается, дальше — диалог Go, проверка окном, тост (fontupload.ts).
  // Работает и после закрытия списка: из компонентов в нём ничего не нужно.
  async function upload() {
    if (busy) return
    open = false
    busy = true
    try {
      await uploadFonts({ fonts: () => store.fonts, notify: (t, k) => store.notify(t, k), pick: (id) => id !== value && onpick(id) })
    } finally {
      busy = false
    }
  }

  async function confirmHide() {
    const a = ask
    ask = null
    if (a) await hideFont(a.id, (t, k) => store.notify(t, k))
  }

  // Положение считаем, когда окно отрисовано и известен размер.
  $effect(() => {
    if (!open || !pop || !btn) return
    const a = btn.getBoundingClientRect()
    placed = placePopover(
      { left: a.left, top: a.top, width: a.width, height: a.height },
      { width: pop.offsetWidth, height: pop.offsetHeight },
      { width: window.innerWidth, height: window.innerHeight },
    )
  })

  async function reveal() {
    await tick()
    pop?.querySelector('[aria-selected="true"]')?.scrollIntoView?.({ block: 'nearest' })
  }

  // Набор букв: переход к гарнитуре по началу имени; пауза > 0,8 с начинает набор заново.
  function typeAhead(ch: string) {
    const now = Date.now()
    typed = now - typedAt > 800 ? ch : typed + ch
    typedAt = now
    const i = findByPrefix(
      flat.map((f) => f.name),
      typed,
      active,
    )
    if (i >= 0) active = i
  }

  // Ctrl/Alt/Meta не трогаем: горячие клавиши программы остаются рабочими.
  function onKey(e: KeyboardEvent) {
    if (!open || e.ctrlKey || e.altKey || e.metaKey) return
    const moved = moveActive(e.key, active, flat.length)
    let handled = true
    if (moved !== null) active = moved
    else if (e.key === 'Escape') close()
    else if (e.key === 'Enter') pick(active)
    else if (e.key.length === 1 && e.key !== ' ') typeAhead(e.key)
    else handled = false
    if (!handled) return
    e.stopPropagation()
    e.preventDefault()
    if (open) void reveal()
  }

  function onPointer(e: PointerEvent) {
    if (!open) return
    const t = e.target as Node
    if (!pop?.contains(t) && !btn?.contains(t)) open = false
  }
</script>

<svelte:window
  onkeydowncapture={onKey}
  onpointerdowncapture={onPointer}
  onscrollcapture={(e) => open && !pop?.contains(e.target as Node) && (open = false)}
  onresize={() => (open = false)}
/>

<!-- Кнопки не забирают фокус у редактора: курсор остаётся в тексте -->
<button
  bind:this={btn}
  class="fp"
  data-guide="font"
  type="button"
  title={shown.missing ? MISSING_TIP : 'Шрифт'}
  aria-label="Шрифт"
  aria-haspopup="listbox"
  aria-expanded={open}
  onmousedown={(e) => e.preventDefault()}
  onclick={() => (open ? close() : show())}
>
  <span class="cur" class:missing={shown.missing} style:font-family={curFamily}>{shown.name}</span>
  <Icon name="caret-down" size={14} />
</button>

{#if open}
  <div
    bind:this={pop}
    class="pop"
    use:portal
    data-chrome
    data-text-ui
    role="listbox"
    aria-label="Шрифт"
    aria-activedescendant={`fp-${flat[active]?.id}`}
    tabindex="-1"
    style:min-width="{Math.max(width, 260)}px"
    style:left="{placed?.left ?? 0}px"
    style:top="{placed?.top ?? 0}px"
    style:visibility={placed ? 'visible' : 'hidden'}
    onmousedown={(e) => e.preventDefault()}
  >
    {#each groups as g, gi (g.id)}
      <div role="group" aria-label={g.title}>
        <div class="cap">{g.title}</div>
        {#each g.fonts as f, i (f.id)}
          {@const idx = offsets[gi] + i}
          <div
            id={`fp-${f.id}`}
            class="opt"
            class:act={idx === active}
            role="option"
            tabindex="-1"
            aria-selected={f.id === value}
            onmousemove={() => (active = idx)}
            onclick={() => pick(idx)}
            onkeydown={null}
          >
            <span class="sample" style:font-family={css(f)}>{sampleOf(f)}</span>
            <span class="meta">
              {#if f.noCyr}<span class="lat">{LAT_ONLY}</span>{/if}
              <span class="name">{f.name}</span>
              {#if f.group === 'my'}
                <!-- × только у своих; щелчок не выбирает строку -->
                <button
                  type="button"
                  class="x"
                  tabindex="-1"
                  title={HIDE_TITLE}
                  aria-label="{HIDE_TITLE}: {f.name}"
                  onclick={(e) => {
                    e.stopPropagation()
                    open = false
                    ask = { id: f.id, name: f.name }
                  }}><Icon name="x" size={12} /></button
                >
              {/if}
            </span>
          </div>
        {/each}
      </div>
    {/each}
    <!-- Строка вне навигации: список закрывается, диалог Go открывается из FontPicker -->
    <button type="button" class="up" disabled={busy} onclick={upload}>{UPLOAD_LABEL}</button>
  </div>
{/if}

{#if ask}
  <Modal title="Убрать из библиотеки?" onclose={() => (ask = null)}>
    <p>{hideText(ask.name)}</p>
    <div class="actions">
      <button class="ghost" onclick={() => (ask = null)}>Отмена</button>
      <button class="primary" data-autofocus onclick={confirmHide}>Убрать</button>
    </div>
  </Modal>
{/if}

<style>
  .fp {
    width: 100%;
    height: 26px;
    padding: 0 6px 0 8px;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 6px;
    font-size: 14px;
    background: #3a3a3e;
    color: var(--text);
  }
  .cur {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .pop {
    position: fixed;
    z-index: 90;
    max-height: 60vh;
    overflow-y: auto;
    padding: 4px 0;
    background: var(--panel);
    border: 1px solid var(--line);
    border-radius: 8px;
    box-shadow: 0 10px 32px rgba(0, 0, 0, 0.5);
    outline: none;
  }
  .cap {
    padding: 8px 12px 4px;
    font-size: 11px;
    color: var(--muted);
    text-transform: uppercase;
    letter-spacing: 0.04em;
  }
  .opt {
    display: flex;
    align-items: baseline;
    justify-content: space-between;
    gap: 16px;
    padding: 4px 12px;
    cursor: pointer;
  }
  .opt.act {
    background: #3a3a3e;
  }
  .opt[aria-selected='true'] .name {
    color: var(--accent);
  }
  .sample {
    font-size: 16px;
    font-weight: 400;
    font-style: normal;
    white-space: nowrap;
  }
  .meta {
    flex: none;
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 11px;
    color: var(--muted);
  }
  .lat {
    font-size: 10px;
    opacity: 0.65;
  }
  /* × места не занимает, пока строка не под курсором: ширина списка не прыгает */
  .x {
    width: 18px;
    height: 18px;
    padding: 0;
    display: grid;
    place-items: center;
    border-radius: 4px;
    background: rgba(17, 17, 17, 0.8);
    visibility: hidden;
  }
  .opt:hover .x,
  .opt.act .x {
    visibility: visible;
  }
  .cur.missing {
    color: var(--accent);
  }
  .up {
    display: block;
    width: calc(100% - 16px);
    margin: 4px 8px 2px;
    padding: 6px 8px;
    border-top: 1px solid var(--line);
    border-radius: 0 0 6px 6px;
    text-align: left;
    font-size: 13px;
    color: var(--text);
    background: transparent;
  }
  .up:hover:not(:disabled) {
    background: #3a3a3e;
  }
  .actions {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
    margin-top: 12px;
  }
</style>
