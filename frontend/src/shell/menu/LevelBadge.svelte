<script lang="ts">
  import { prefs } from '../../base/prefs/prefs.svelte'
  import { ALL_LEVELS, LEVEL_HINT, LEVEL_TITLE, customHint, type Level } from '../../base/prefs/levels'
  import { run } from './commands'

  // Значок уровня в шапке (спека 010): «Уровень: Простой ▾», список из четырёх уровней и «Уровни и функции…».
  // Esc и щелчок мимо закрывают; выбор идёт через prefs.set (понижение снимает выделение и режим фона).
  let open = $state(false)
  let btn = $state<HTMLElement>()
  let right = $state(0)
  let top = $state(0)

  function toggle() {
    if (!open && btn) {
      const r = btn.getBoundingClientRect()
      right = Math.max(8, window.innerWidth - r.right)
      top = r.bottom + 4
    }
    open = !open
  }

  // У «Своего» подсказка называет основу набора.
  const hint = (l: Level): string => (l === 'custom' && prefs.custom ? customHint(prefs.custom.base) : LEVEL_HINT[l])

  function pick(l: Level) {
    open = false
    void prefs.set(l)
  }

  function outside(e: PointerEvent) {
    if (open && !(e.target instanceof Element && e.target.closest('[data-level-badge]'))) open = false
  }
  function key(e: KeyboardEvent) {
    if (open && e.key === 'Escape') {
      e.stopPropagation()
      open = false
    }
  }
</script>

<svelte:window onpointerdowncapture={outside} onkeydowncapture={key} onresize={() => (open = false)} onblur={() => (open = false)} />

<span class="wrap" data-level-badge data-guide="level-badge" data-chrome data-text-ui>
  <button
    class="ghost"
    bind:this={btn}
    aria-haspopup="menu"
    aria-expanded={open}
    onclick={toggle}
    title="Уровень интерфейса: сколько инструментов показано. Данные альбома от него не зависят"
    >Уровень: {LEVEL_TITLE[prefs.level]} ▾</button
  >
  {#if open}
    <div class="drop" role="menu" style:right="{right}px" style:top="{top}px">
      {#each ALL_LEVELS as l (l)}
        <button role="menuitemradio" aria-checked={l === prefs.level} onclick={() => pick(l)}>
          <span class="mark" aria-hidden="true">{l === prefs.level ? '✓' : ''}</span>
          <span class="txt"><b>{LEVEL_TITLE[l]}</b><small>{hint(l)}</small></span>
        </button>
      {/each}
      <div class="sep" role="separator"></div>
      <button
        role="menuitem"
        onclick={() => {
          open = false
          run('compareLevels')
        }}><span class="mark"></span><span class="txt">Уровни и функции…</span></button
      >
    </div>
  {/if}
</span>

<style>
  .wrap {
    flex: none;
  }
  .drop {
    position: fixed;
    z-index: 70;
    /* Описания уровней переносятся по словам и не вылезают за край списка. */
    width: 360px;
    max-width: calc(100vw - 16px);
    padding: 4px;
    background: rgba(30, 30, 32, 0.97);
    border: 1px solid var(--line);
    border-radius: 6px;
    box-shadow: 0 6px 20px rgba(0, 0, 0, 0.5);
    font-size: 13px;
  }
  .drop button {
    width: 100%;
    height: auto;
    min-height: 28px;
    padding: 4px 10px 4px 4px;
    display: flex;
    align-items: flex-start;
    gap: 6px;
    text-align: left;
    background: transparent;
    color: var(--text);
    border-radius: 4px;
  }
  .drop button:hover,
  .drop button:focus-visible {
    background: #38383c;
    color: #fff;
    outline: none;
  }
  .mark {
    flex: none;
    width: 16px;
    text-align: center;
    color: var(--accent);
  }
  .txt {
    white-space: normal;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }
  small {
    font-size: 11px;
    line-height: 1.3;
    color: var(--muted);
  }
  .sep {
    height: 1px;
    margin: 4px 2px;
    background: var(--line);
  }
</style>
