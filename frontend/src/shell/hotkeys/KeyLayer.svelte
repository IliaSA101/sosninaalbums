<script lang="ts">
  import { isMac } from '../../base/core/platform'
  import { modName } from '../../base/keys/chordtext'
  import { keysUi } from './keysui.svelte'

  // «Слой: Ctrl Shift Alt» (074): какие модификаторы показывает карта; зажатые клавиши на клавиатуре тоже переключают слой.
  const mac = isMac()
  const LAYERS = [
    ['mod', 'Mod'],
    ['shift', 'Shift'],
    ['alt', 'Alt'],
  ] as const
</script>

<span class="mods" title="Слой карты; зажатая клавиша на клавиатуре тоже переключает слой">
  Слой:
  {#each LAYERS as [k, n] (k)}
    <button class:on={keysUi.mods[k]} aria-pressed={keysUi.mods[k]} onclick={() => keysUi.toggleMod(k)}>{modName(n, mac)}</button>
  {/each}
</span>

<style>
  .mods {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    color: var(--muted);
    font-size: 12px;
  }
  button {
    height: 28px;
    padding: 0 10px;
    background: #3a3a3e;
  }
  button.on {
    background: var(--accent);
    color: #1e1e1e;
    font-weight: 600;
  }
</style>
