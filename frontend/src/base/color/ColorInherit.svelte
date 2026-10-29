<script lang="ts">
  import { CUSTOM_NAME, nameOf } from './palette'

  // Строка «Как в альбоме» в окне цвета листа (115): образец цвета альбома, имя из наборов и HEX серым, ✓ — если своего цвета нет.
  // Наведение и фокус показывают цвет альбома на развороте (как образцы), щелчок решает родитель (ColorPicker).
  let {
    hex,
    on,
    onpick,
    onpreview,
  }: {
    hex: string
    on: boolean
    onpick: () => void
    onpreview: (hex: string | null) => void
  } = $props()

  const name = $derived(nameOf(hex))
  const sub = $derived(name === CUSTOM_NAME ? hex : `${name} · ${hex}`)
</script>

<button
  type="button"
  class="inherit"
  class:on
  aria-pressed={on}
  onclick={onpick}
  onpointerenter={() => onpreview(hex)}
  onpointerleave={() => onpreview(null)}
  onfocus={() => onpreview(hex)}
  onblur={() => onpreview(null)}
>
  <span class="sw" style:background={hex}></span>
  <span class="txt">
    <b>Как в альбоме</b>
    <span class="sub">{sub}</span>
  </span>
  {#if on}<span class="ok" aria-hidden="true">✓</span>{/if}
</button>

<style>
  .inherit {
    display: flex;
    align-items: center;
    gap: 8px;
    width: 100%;
    box-sizing: border-box;
    height: auto;
    margin-top: 8px;
    padding: 4px 8px 4px 4px;
    text-align: left;
    background: transparent;
    border: 1px solid var(--line);
    border-radius: 6px;
    color: var(--text);
  }
  .inherit:hover {
    background: #3a3a3e;
  }
  .inherit.on {
    border-color: var(--accent);
  }
  .sw {
    flex: none;
    width: 26px;
    height: 26px;
    border-radius: 5px;
    border: 1px solid rgba(255, 255, 255, 0.12);
  }
  .txt {
    flex: 1;
    min-width: 0;
    line-height: 1.2;
  }
  .txt b {
    display: block;
    font-weight: 600;
    font-size: 12px;
  }
  .sub {
    display: block;
    color: var(--muted);
    font-family: Consolas, monospace;
    font-size: 11px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .ok {
    flex: none;
    color: var(--accent);
    font-size: 14px;
  }
</style>
