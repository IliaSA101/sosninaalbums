<script lang="ts">
  import { placesel } from '../places/placesel.svelte'
  import { miniContext } from '../places/minipanel'
  import { store } from '../../base/core/store.svelte'
  import Icon from '../../base/ui/Icon.svelte'

  // «Зеркало ↔ / ↕» в карточке фото (спека 009 U4): подсвечено, если отражено у всех выделенных.
  // Решение «всем или никому» принимает Go (SetFlip).
  const c = $derived(miniContext(store.spread, placesel.selected))
</script>

<div class="lbl">Зеркало</div>
<div class="seg" role="group" aria-label="Зеркало" data-guide="flip-toggle">
  <button class:on={c.flipH} aria-pressed={c.flipH} title="Отразить по горизонтали" onclick={() => placesel.flip('h')}
    ><Icon name="flip-h" size={14} />↔</button
  >
  <button class:on={c.flipV} aria-pressed={c.flipV} title="Отразить по вертикали" onclick={() => placesel.flip('v')}
    ><Icon name="flip-v" size={14} />↕</button
  >
</div>

<style>
  .lbl {
    margin-top: 10px;
    font-size: 12px;
    color: var(--muted);
  }
  .seg {
    display: flex;
    margin-top: 4px;
    border-radius: 5px;
    overflow: hidden;
    gap: 1px;
  }
  .seg button {
    flex: 1;
    height: 26px;
    padding: 0 6px;
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 6px;
    font-size: 12px;
    border-radius: 0;
    background: #38383c;
    color: #cfcfd4;
  }
  .seg button.on {
    background: var(--accent);
    color: #1e1e1e;
  }
  .seg button.on:hover {
    background: #ffe53a;
  }
</style>
