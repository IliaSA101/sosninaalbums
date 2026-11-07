<script lang="ts">
  import { tick } from 'svelte'
  import Modal from '../../base/modal/Modal.svelte'
  import { watchSize } from '../../base/ui/size'
  import { keymap } from '../../base/keys/keymap.svelte'
  import { modes } from '../../base/prefs/view.svelte'
  import type { KeysSplit } from '../../base/prefs/keyswin'
  import { isTextField } from '../menu/keys'
  import KeyAsk from './KeyAsk.svelte'
  import KeyBoard from './KeyBoard.svelte'
  import KeyCard from './KeyCard.svelte'
  import KeyLayer from './KeyLayer.svelte'
  import KeyLegend from './KeyLegend.svelte'
  import KeyList from './KeyList.svelte'
  import Splitter from './Splitter.svelte'
  import { ownsKeys } from './keyrec'
  import { dragSplit, fitBoard, resetSplit, tracksStyle, U_MINI, type SplitKey } from './keyslayout'
  import { keysUi } from './keysui.svelte'

  // Окно «Горячие клавиши» (074-29/30, макет Г): карта, список и карточка в одной сетке, два вида — «Карта крупно»
  // (как вариант А) и «Список крупно» (как Б); границы тянутся, двойной щелчок — «авто». Вид и доли помнятся (modes).
  // Пока окно открыто, нажатия идут на карту, а не в программу (правило окон 074-18), а при записи их забирает запись.
  const mode = $derived(modes.keysMode)
  let live = $state.raw<KeysSplit | null>(null) // доли в момент перетаскивания; в настройки — на отпускании
  const split = $derived(live ?? modes.keysSplit)

  // Размеры блоков из DOM (ResizeObserver): по ним считается размер клавиш.
  let m = $state({ bodyW: 0, bodyH: 0, paneW: 0, paneH: 0, headH: 0, legendH: 0 })
  const narrow = $derived(m.bodyW > 0 && m.bodyW < 1010) // узкое окно: легенда уже, а в «Списке крупно» мельче шрифт
  const fit = $derived(
    fitBoard({
      mode,
      autoRows: split.ra === null,
      paneW: m.paneW,
      paneH: m.paneH,
      bodyH: m.bodyH,
      headH: m.headH,
      legendH: m.legendH,
      legendW: narrow ? 190 : 230,
    }),
  )
  const hsKey = $derived<SplitKey>(mode === 'map' ? 'ra' : 'rh')

  const move = (k: SplitKey) => (p: { x: number; y: number }, b: { w: number; h: number }) => (live = dragSplit(k, split, p, b))
  function end() {
    if (live) modes.setKeysSplit(live)
    live = null
  }
  const reset = (k: SplitKey) => () => modes.setKeysSplit(resetSplit(k, modes.keysSplit))

  // Нажатия окна на захвате: окно открыто раньше обработчика программы и забирает клавиши для карты и записи.
  const targetOf = (e: Event) => (e.target instanceof Element ? e.target : null)
  const inText = (e: Event) => isTextField(targetOf(e))
  function keydown(e: KeyboardEvent) {
    // Enter/Пробел/стрелки/Home/End на кнопке, флажке и в списке — самому элементу (вне записи).
    if (keysUi.keydown(e, inText(e), ownsKeys(targetOf(e))) === 'eat') {
      e.preventDefault()
      e.stopImmediatePropagation()
    }
  }
  // Из поля поиска ↓ входит в список: выбирается первая строка (↑/↓ внутри поля больше ничего не делают).
  let q = $state<HTMLInputElement>()
  let list = $state<KeyList>()
  function searchKey(e: KeyboardEvent) {
    if (e.key !== 'ArrowDown' || e.ctrlKey || e.altKey || e.shiftKey || e.metaKey || e.isComposing) return
    e.preventDefault()
    list?.focusFirst()
  }

  // Кнопка гаснет, а строка «Вернуть» исчезает вместе с фокусом: возвращаем его на поиск, чтобы он не ушёл из окна.
  async function keepFocus(act: Promise<void>) {
    await act
    await tick()
    const a = document.activeElement
    if (!a || a === document.body || !a.isConnected || (a instanceof HTMLButtonElement && a.disabled)) q?.focus()
  }

  const keyup = (e: KeyboardEvent) => keysUi.keyup(e, inText(e))

  // Щелчок мимо чипа отменяет запись; щелчок по чипу (другому или тому же) начинает её заново.
  function press(e: PointerEvent) {
    if (keysUi.rec && !(e.target instanceof Element && e.target.closest('[data-chip]'))) keysUi.cancelRec()
  }
</script>

<svelte:window onkeydowncapture={keydown} onkeyup={keyup} onblur={() => keysUi.blur()} />

{#snippet headExtra()}
  <span class="vsw" role="group" aria-label="Вид окна">
    <button class:on={mode === 'map'} aria-pressed={mode === 'map'} onclick={() => modes.setKeysMode('map')}>Карта крупно</button>
    <button class:on={mode === 'list'} aria-pressed={mode === 'list'} onclick={() => modes.setKeysMode('list')}>Список крупно</button>
  </span>
{/snippet}

{#if keysUi.open}
  <Modal title="Горячие клавиши" size="large" onclose={() => keysUi.close()} onescape={() => keysUi.escape()} {headExtra}>
    <div class="keyswin" role="presentation" onpointerdowncapture={press}>
      <div class="bar">
        <input
          class="q"
          type="search"
          placeholder="Найти команду или клавишу"
          aria-label="Поиск по командам"
          autocomplete="off"
          bind:value={keysUi.query}
          bind:this={q}
          onkeydown={searchKey}
        />
        <label class="onlyown"><input type="checkbox" bind:checked={keysUi.onlyOwn} />Только свои</label>
        {#if mode === 'map'}<KeyLayer />{/if}
        <button
          class="ghost"
          disabled={keymap.view.changed === 0}
          title={keymap.view.changed === 0 ? 'Своих сочетаний нет' : undefined}
          onclick={() => void keepFocus(keysUi.resetAll())}
        >
          Сбросить всё как было
        </button>
      </div>

      <div
        class="body"
        data-m={mode}
        class:narrow
        style={tracksStyle(split, fit.autoH)}
        {@attach (n) => watchSize(n, (w, h) => ((m.bodyW = w), (m.bodyH = h)))}
      >
        <div
          class="p-map"
          class:below={fit.below}
          style:--lw="{narrow ? 190 : 230}px"
          {@attach (n) => watchSize(n, (w, h) => ((m.paneW = w), (m.paneH = h)))}
        >
          {#if mode === 'list'}
            <div class="mp-head" {@attach (n) => watchSize(n, (_w, h) => (m.headH = h))}>
              <KeyLayer />
              <div class="lg line"><KeyLegend line /></div>
            </div>
          {:else}
            <!-- Невидимая копия легенды «под картой»: по её высоте решается, где легенде быть -->
            <div class="probe lg line" aria-hidden="true" {@attach (n) => watchSize(n, (_w, h) => (m.legendH = h))}>
              <KeyLegend line />
            </div>
          {/if}
          <div class="mp-board"><KeyBoard unit={fit.u} mini={fit.u < U_MINI} /></div>
          {#if mode === 'map'}<div class="lg" class:line={fit.below}><KeyLegend line={fit.below} /></div>{/if}
        </div>
        <Splitter area="hs" dir="row" onmove={move(hsKey)} onend={end} onreset={reset(hsKey)} />
        {#if mode === 'list'}
          <Splitter area="vs" dir="col" onmove={move('rw')} onend={end} onreset={reset('rw')} />
        {/if}
        <div class="p-list"><KeyList {mode} bind:this={list} /></div>
        <div class="p-card"><KeyCard /></div>
      </div>

      <p class="status" aria-live="polite">
        {#if keysUi.status}
          {keysUi.status.text} · <button class="link" onclick={() => void keepFocus(keysUi.restore())}>Вернуть</button>
        {/if}
        {#each keysUi.notes as n (n)}<span class="note">{n}</span>{/each}
      </p>
    </div>
  </Modal>
{/if}

<KeyAsk />

<style>
  /* Окно крупное и фиксированной высоты: блоки делят её между собой */
  :global(.box.large):has(.keyswin) {
    height: min(724px, calc(100vh - 32px));
    display: flex;
    flex-direction: column;
  }
  .keyswin {
    flex: 1 1 0;
    min-height: 0;
    display: flex;
    flex-direction: column;
  }
  .vsw {
    display: inline-flex;
    gap: 2px;
    padding: 2px;
    background: var(--panel2);
    border: 1px solid var(--line);
    border-radius: 7px;
    font-weight: 400;
  }
  .vsw button {
    height: 24px;
    padding: 0 12px;
    font-size: 12px;
    background: transparent;
    color: var(--muted);
    border-radius: 5px;
  }
  .vsw button:hover:not(.on) {
    background: #38383c;
    color: var(--text);
  }
  .vsw button.on {
    background: var(--accent);
    color: #1e1e1e;
    font-weight: 600;
  }
  .bar {
    display: flex;
    align-items: center;
    gap: 12px;
    flex: none;
    margin-bottom: 8px;
  }
  .q {
    flex: 1;
    min-width: 160px;
    height: 32px;
    padding: 0 10px;
    font: inherit;
    color: var(--text);
    background: #3a3a3e;
    border: 1px solid var(--line);
    border-radius: 6px;
    user-select: text;
  }
  .q:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: 0;
  }
  .onlyown {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    cursor: pointer;
    white-space: nowrap;
    color: var(--muted);
  }

  /* Тело: сетка из трёх блоков, дорожки задаёт keyslayout.ts через --rA, --cB, --rB */
  .body {
    flex: 1 1 0;
    min-height: 0;
    display: grid;
    position: relative;
  }
  .body[data-m='map'] {
    grid-template-columns: minmax(0, 1fr) minmax(280px, 380px);
    grid-template-rows: var(--rA);
    grid-template-areas: 'map map' 'hs hs' 'list card';
  }
  .body[data-m='list'] {
    grid-template-columns: var(--cB);
    grid-template-rows: var(--rB);
    grid-template-areas: 'list vs card' 'list vs hs' 'list vs map';
  }
  .p-map {
    grid-area: map;
    min-width: 0;
    min-height: 0;
    overflow: hidden;
    position: relative;
    display: flex;
    flex-direction: column;
    gap: 6px;
  }
  /* «Карта крупно» — как А: карта слева на всю ширину, легенда столбцом справа; при запасе по высоте легенда уходит под карту */
  .body[data-m='map'] .p-map {
    flex-direction: row;
    align-items: center;
    gap: 18px;
  }
  .body[data-m='map'] .p-map.below {
    flex-direction: column;
    align-items: stretch;
    gap: 6px;
  }
  .p-list {
    grid-area: list;
    min-width: 0;
    min-height: 0;
  }
  .p-card {
    grid-area: card;
    min-width: 0;
    min-height: 0;
    overflow-y: auto;
    scrollbar-width: thin;
    padding-right: 4px;
  }
  .body[data-m='map'] .p-card {
    padding-left: 14px;
    border-left: 1px solid var(--line);
  }
  .body[data-m='map'] .p-list {
    padding-right: 8px;
  }
  .body[data-m='list'] .p-list {
    padding-right: 4px;
  }
  .mp-head {
    flex: none;
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 4px 14px;
  }
  .mp-board {
    flex: 1 1 0;
    min-height: 0;
    min-width: 0;
    overflow: hidden;
    display: flex;
    align-items: center;
    justify-content: center;
  }
  .body[data-m='map'] .mp-board {
    justify-content: flex-start;
  }

  /* Легенда: размер шрифта и место задаёт окно, рисует KeyLegend */
  .lg {
    font-size: 11px;
  }
  .narrow .lg {
    font-size: 10px;
  }
  .body[data-m='map'] .lg:not(.line) {
    flex: none;
    width: var(--lw);
    align-self: flex-start;
    padding-top: 4px;
    font-size: 12px;
  }
  .body[data-m='map'].narrow .lg:not(.line) {
    font-size: 11px;
  }
  .probe {
    position: absolute;
    left: 0;
    right: 0;
    top: 0;
    visibility: hidden;
    pointer-events: none;
  }

  .status {
    flex: none;
    min-height: 20px;
    margin: 6px 0 0;
    color: var(--muted);
  }
  .note {
    display: block;
    font-size: 12px;
    font-style: italic;
  }
  .link {
    height: auto;
    padding: 0;
    background: transparent;
    color: var(--accent);
    white-space: normal;
    text-align: left;
  }
  .link:hover:not(:disabled) {
    background: transparent;
    text-decoration: underline;
  }
</style>
