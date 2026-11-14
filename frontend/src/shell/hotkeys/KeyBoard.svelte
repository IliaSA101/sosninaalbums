<script lang="ts">
  import Icon from '../../base/ui/Icon.svelte'
  import { isMac } from '../../base/core/platform'
  import { chordText, modName } from '../../base/keys/chordtext'
  import { keymap } from '../../base/keys/keymap.svelte'
  import { fitCmd, MAIN, NAV, type BoardRow, type Cell } from './board'
  import { dimOthers, keyCell } from './keysel'
  import { keysUi } from './keysui.svelte'

  // Карта клавиатуры (074, макет Г): на клавише латиница, бледная русская буква, короткое имя команды, цвет — группа,
  // замок — служебная, «+N» — на клавише ещё команды. Клавиши Ctrl/Shift/Alt включают слой карты.
  // unit — размер единицы клавиши в px (считает окно по свободному месту), mini — клавиши мельче подписей.
  let { unit, mini }: { unit: number; mini: boolean } = $props()

  const mac = isMac()
  const modLabel = (m: 'mod' | 'alt' | 'shift'): string =>
    m === 'shift' ? (mini ? 'Shift' : '⇧ Shift') : modName(m === 'mod' ? 'Mod' : 'Alt', mac)
  const dim = $derived(dimOthers(keymap.view, keysUi.sel, keysUi.mods))

  function tip(chord: string, why: string, labels: string[]): string {
    return [chordText(chord, mac) + (why ? ` — служебная: ${why}` : ''), ...labels].join('\n')
  }
</script>

{#snippet row(r: BoardRow)}
  <div class="krow" class:fn={r.fn}>
    {#each r.cells as c, i (i)}
      {@render cell(c)}
    {/each}
  </div>
{/snippet}

{#snippet cell(c: Cell)}
  {#if c.kind === 'gap'}
    <div class="gap" style:--w={c.w}></div>
  {:else if c.kind === 'dead'}
    <div class="kw" style:--w={c.w}><div class="k dead" aria-hidden="true">{mini ? '' : c.label}</div></div>
  {:else if c.kind === 'mod'}
    <div class="kw" style:--w={c.w}>
      <button
        class="k mod"
        class:on={keysUi.mods[c.mod]}
        aria-pressed={keysUi.mods[c.mod]}
        title="Слой карты"
        onclick={() => keysUi.toggleMod(c.mod)}
      >
        {modLabel(c.mod)}
      </button>
    </div>
  {:else}
    {@const k = keyCell(keysUi.ctx, c.code, keysUi.mods, keysUi.sel, keysUi.cur, keymap.view)}
    {@const top = k.cmds[0]}
    <div class="kw" style:--w={c.w}>
      <button
        class="k"
        class:lit={!!top}
        class:lk2={!!k.why}
        class:sel={k.selected}
        class:hl={k.hl}
        style:--g={top ? `var(--grp-${top.group}, var(--grp-other))` : 'transparent'}
        title={tip(
          k.chord,
          k.why,
          k.cmds.map((e) => e.label),
        )}
        aria-pressed={k.selected}
        onclick={() => keysUi.selectKey(k.chord)}
      >
        <span class="en">{c.en}</span>
        {#if c.ru}<span class="ru">{c.ru}</span>{/if}
        {#if top}{@const f = fitCmd(top.short, c.w, unit)}<span class="cmd" style:font-size="{f.font}px">{f.text}</span>{/if}
        {#if k.cmds.length > 1}<span class="more">+{k.cmds.length - 1}</span>{/if}
        {#if k.why}<span class="lk"><Icon name="lock" size={10} /></span>{/if}
      </button>
    </div>
  {/if}
{/snippet}

<div class="kb" class:mini class:has-hl={dim} style:--u="{unit}px">
  <div class="kcol">
    {#each MAIN as r, i (i)}{@render row(r)}{/each}
  </div>
  <div class="kcol">
    {#each NAV as r, i (i)}{@render row(r)}{/each}
  </div>
</div>

<style>
  .kb {
    display: flex;
    gap: calc(var(--u) * 0.25);
    flex: none;
  }
  .kcol {
    display: flex;
    flex-direction: column;
  }
  .krow {
    display: flex;
    height: var(--u);
  }
  .krow.fn {
    height: calc(var(--u) * 0.8);
    margin-bottom: calc(var(--u) * 0.25);
  }
  .kw {
    width: calc(var(--w) * var(--u));
    height: 100%;
    padding: 1.5px;
    flex: none;
  }
  .gap {
    width: calc(var(--w) * var(--u));
    flex: none;
  }
  .k {
    position: relative;
    display: block;
    width: 100%;
    height: 100%;
    padding: 0;
    border-radius: 5px;
    border: 1px solid #3a3a3e;
    background: #2f2f33;
    color: var(--text);
    overflow: hidden;
    text-align: left;
    white-space: normal;
    transition: none;
  }
  .k:hover:not(:disabled) {
    background: #3a3a3f;
    border-color: #5a5a60;
  }
  .en {
    position: absolute;
    left: 4px;
    top: 2px;
    font-size: clamp(8px, calc(var(--u) * 0.22), 12px);
    color: #c9c9ce;
    line-height: 1;
  }
  .ru {
    position: absolute;
    right: 4px;
    bottom: 2px;
    font-size: clamp(8px, calc(var(--u) * 0.2), 11px);
    color: #6f6f78;
    line-height: 1;
  }
  .cmd {
    position: absolute;
    left: 2px;
    right: 2px;
    top: 34%;
    text-align: center;
    line-height: 1.05;
    font-weight: 600;
    color: #fff;
    overflow: hidden;
    max-height: 46%;
    /* Только по словам: слово не рвётся и не переносится дефисом (размер и усечение — fitCmd) */
    word-break: normal;
    overflow-wrap: normal;
    hyphens: manual;
  }
  .more {
    position: absolute;
    right: 2px;
    top: 2px;
    font-size: 9px;
    line-height: 1;
    padding: 1px 3px;
    border-radius: 6px;
    background: rgba(0, 0, 0, 0.55);
    color: var(--text);
  }
  .lk {
    position: absolute;
    left: 3px;
    bottom: 3px;
    display: inline-flex;
    color: var(--muted);
  }
  .k.lit {
    background: color-mix(in srgb, var(--g) 40%, #202022);
    border-color: color-mix(in srgb, var(--g) 75%, #3a3a3e);
  }
  .k.lit:hover:not(:disabled) {
    background: color-mix(in srgb, var(--g) 52%, #202022);
  }
  .k.lk2 {
    background: #151517;
    border: 1px dashed #4c4c53;
    color: var(--muted);
  }
  .k.lk2 .cmd {
    color: #b5b5bb;
    font-weight: 500;
  }
  .k.lk2.lit {
    border-color: color-mix(in srgb, var(--g) 55%, #4c4c53);
  }
  /* Выбранная клавиша — рамка; клавиши выбранной команды — тонкая внутренняя; остальные приглушены, как при выборе в списке */
  .k.sel {
    outline: 2px solid var(--accent);
    outline-offset: -1px;
    z-index: 1;
  }
  .k.hl {
    box-shadow: inset 0 0 0 2px var(--accent);
  }
  .k.sel.hl {
    outline: none;
  }
  .has-hl .k.lit:not(.hl):not(.sel) {
    opacity: 0.4;
  }
  .k.mod,
  .k.dead {
    display: grid;
    place-items: center;
    background: #26262a;
    color: var(--muted);
    font-size: clamp(8px, calc(var(--u) * 0.22), 12px);
    text-align: center;
  }
  .k.dead {
    cursor: default;
    opacity: 0.6;
  }
  .k.dead:hover {
    background: #26262a;
    border-color: #3a3a3e;
  }
  .k.mod.on {
    border-color: var(--accent);
    color: var(--accent);
    background: #3a3a3e;
  }
  /* На мелкой карте подписи команд нечитаемы — остаются буквы и цвет группы */
  .mini .cmd,
  .mini .ru,
  .mini .more,
  .mini .lk {
    display: none;
  }
  .mini .en {
    left: 0;
    right: 0;
    top: 50%;
    transform: translateY(-50%);
    text-align: center;
  }
  .mini .k {
    border-radius: 4px;
  }
  .mini .krow.fn .en {
    font-size: 7px;
  }
</style>
