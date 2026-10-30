<script lang="ts">
  import { store } from '../../base/core/store.svelte'
  import { ASSOC_ASK, assocUi } from './assoc.svelte'

  // Вопрос о связи .salbum (112) в стартовом окне, рядом с «Показывать при запуске». Показывает Go: State.assoc.ask.
</script>

{#if store.assoc.ask}
  <div class="assoc" role="group" aria-label={ASSOC_ASK}>
    <span class="q">{ASSOC_ASK}</span>
    <span class="btns">
      <button class="accent" disabled={assocUi.busy} onclick={() => assocUi.yes()}>Да</button>
      <button disabled={assocUi.busy} onclick={() => assocUi.later()}>Не сейчас</button>
      <button disabled={assocUi.busy} onclick={() => assocUi.never()}>Не спрашивать</button>
    </span>
  </div>
{/if}

<style>
  .assoc {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    padding: 6px 0;
  }
  .btns {
    display: flex;
    gap: 6px;
    flex: none;
  }
</style>
