<script lang="ts">
  import Icon from '../../base/ui/Icon.svelte'
  import { tipped } from '../../base/ui/tip.svelte'

  // Значок виньетки в левом верхнем углу плитки (лента, «Все развороты»); зеркало LowDpiBadge справа.
  // Лежит в отступе плитки (content-visibility обрежет всё за её краем). Родитель — position: relative.
  let { kind }: { kind?: string } = $props()
</script>

{#if kind === 'vignette'}
  <span class="mark" data-guide="vig-mark" role="img" aria-label="Виньетка" use:tipped={{ text: 'Виньетка' }}
    ><Icon name="vignette" size={10} stroke={2.4} /></span
  >
{/if}

<style>
  .mark {
    position: absolute;
    top: 1px;
    left: 1px;
    width: 14px;
    height: 14px;
    border-radius: 50%;
    background: var(--accent);
    color: #1e1e1e;
    display: grid;
    place-items: center;
    z-index: 4;
  }
</style>
