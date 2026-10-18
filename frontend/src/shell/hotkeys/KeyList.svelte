<script lang="ts">
  import Icon from '../../base/ui/Icon.svelte'
  import { isMac } from '../../base/core/platform'
  import { chordText } from '../../base/keys/chordtext'
  import { keymap } from '../../base/keys/keymap.svelte'
  import type { KeysMode } from '../../base/prefs/keyswin'
  import KeyChips from './KeyChips.svelte'
  import { emptyText, groupRows, listed, rowTag } from './keylist'
  import { keysUi } from './keysui.svelte'

  // Список команд по группам (074). «Карта крупно» — строки с чипами сочетаний (как в варианте А), «Список крупно» —
  // компактные строки (как в справочнике F1), сочетания правятся в карточке. Выбранная клавиша карты выделяет здесь строку.
  let { mode }: { mode: KeysMode } = $props()

  const mac = isMac()
  const opts = $derived({ query: keysUi.query, onlyOwn: keysUi.onlyOwn, visible: keysUi.visible, mac })
  const groups = $derived(groupRows(keymap.view, listed(keymap.view, opts)))

  // Строка выбранной клавиши — в поле зрения списка; прокручивается только сам список, а не окно.
  let box = $state<HTMLElement>()
  $effect(() => {
    const s = keysUi.scroll
    if (!s || !box) return
    const row = box.querySelector<HTMLElement>(`[data-id="${s.id}"]`)
    if (!row) return
    const r = row.getBoundingClientRect()
    const b = box.getBoundingClientRect()
    if (r.top < b.top) box.scrollTop -= b.top - r.top + 4
    else if (r.bottom > b.bottom) box.scrollTop += r.bottom - b.bottom + 4
  })

  // Enter и Пробел на строке — как щелчок (окно забирает нажатия для карты, но строка остаётся доступной щелчком).
  const rowKey = (ev: KeyboardEvent, id: string) => {
    if (ev.key === 'Enter' || ev.key === ' ') {
      ev.preventDefault()
      keysUi.selectRow(id)
    }
  }
</script>

<div class="list" class:assigning={keysUi.pend} class:compact={mode === 'list'} bind:this={box} role="listbox" aria-label="Команды">
  {#each groups as g (g.id)}
    <section>
      <h3 style:--g="var(--grp-{g.id}, var(--grp-other))">
        <i></i>{g.title}{#if g.scopeTag}<span class="gs">· {g.scopeTag}</span>{/if}
      </h3>
      {#each g.rows as e (e.id)}
        {@const on = keysUi.cur === e.id}
        {#if mode === 'map'}
          <div
            class="row"
            class:on
            class:own={e.custom}
            class:fx={e.fixed}
            data-id={e.id}
            role="option"
            aria-selected={on}
            tabindex="-1"
            onclick={() => keysUi.selectRow(e.id)}
            onkeydown={(ev) => rowKey(ev, e.id)}
          >
            <div class="nm">
              <span class="t" title={e.label}>{e.label}</span>
              {#if rowTag(e, g)}<span class="sc">{rowTag(e, g)}</span>{/if}
            </div>
            <div class="ch"><KeyChips {e} /></div>
          </div>
        {:else}
          <button
            class="item"
            class:on
            class:own={e.custom}
            data-id={e.id}
            role="option"
            aria-selected={on}
            onclick={() => keysUi.selectRow(e.id)}
          >
            <span class="t" title={e.label}>{e.label}</span>
            <span class="kt">
              {e.keys[0] ? chordText(e.keys[0], mac) : ''}
              {#if e.fixed}<Icon name="lock" size={12} />{/if}
            </span>
          </button>
        {/if}
      {/each}
    </section>
  {:else}
    <p class="empty">{emptyText(opts)}</p>
  {/each}
</div>

<style>
  .list {
    min-height: 0;
    height: 100%;
    overflow-y: auto;
    scrollbar-width: thin;
  }
  .list.assigning {
    outline: 1px dashed var(--accent);
    outline-offset: -1px;
  }
  .list.assigning :is(.row, .item) {
    cursor: copy;
  }
  h3 {
    margin: 10px 0 2px;
    font-size: 11px;
    font-weight: 600;
    color: var(--muted);
    text-transform: uppercase;
    letter-spacing: 0.04em;
    display: flex;
    align-items: center;
    gap: 6px;
  }
  section:first-child h3 {
    margin-top: 0;
  }
  h3 i {
    width: 8px;
    height: 8px;
    border-radius: 2px;
    background: var(--g);
    display: inline-block;
  }
  .gs {
    text-transform: none;
    letter-spacing: 0;
    font-weight: 400;
  }
  .row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 10px;
    padding: 3px 8px;
    min-height: 30px;
    border-radius: 4px;
    cursor: pointer;
  }
  .row:hover {
    background: #38383c;
  }
  .row.on {
    background: #46464b;
  }
  .nm {
    min-width: 0;
    display: flex;
    flex-direction: column;
    line-height: 1.25;
  }
  .t {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .sc {
    font-size: 11px;
    color: var(--muted);
  }
  .fx .t {
    color: var(--muted);
  }
  .ch {
    display: flex;
    align-items: center;
    justify-content: flex-end;
    flex-wrap: wrap;
    gap: 4px;
  }
  /* Своё сочетание — жёлтая точка у названия */
  .own .t::after {
    content: '';
    display: inline-block;
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: var(--accent);
    margin-left: 6px;
    vertical-align: middle;
  }
  /* «Список крупно»: строка компактная, как в справочнике F1 */
  .item {
    width: 100%;
    height: auto;
    min-height: 28px;
    padding: 4px 8px;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
    text-align: left;
    background: transparent;
    white-space: normal;
    border-radius: 4px;
  }
  .item:hover {
    background: #38383c;
  }
  .item.on {
    background: #46464b;
  }
  .kt {
    flex: none;
    font-size: 11px;
    color: var(--muted);
    display: inline-flex;
    align-items: center;
    gap: 4px;
  }
  .empty {
    margin: 8px;
    color: var(--muted);
  }
</style>
