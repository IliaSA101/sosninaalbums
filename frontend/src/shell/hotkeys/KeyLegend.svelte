<script lang="ts">
  import { keymap } from '../../base/keys/keymap.svelte'

  // Легенда карты (074): цвета групп команд и «служебная». line — в строку (под картой и в шапке «Списка крупно»),
  // иначе столбцом в две колонки справа от карты; размеры шрифта по виду и ширине окна задаёт окно (:global у родителя).
  let { line = false }: { line?: boolean } = $props()
</script>

<div class="legend" class:line>
  {#each keymap.view.groups as g (g.id)}
    <span style:--g="var(--grp-{g.id}, var(--grp-other))"><i></i>{g.title}</span>
  {/each}
  <span><i class="lk2i"></i>служебная</span>
</div>

<style>
  .legend {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 2px 10px;
    align-content: start;
    font-size: inherit;
    color: var(--muted);
  }
  .legend.line {
    display: flex;
    flex-wrap: wrap;
    gap: 4px 14px;
  }
  span {
    display: inline-flex;
    align-items: center;
    gap: 4px;
  }
  i {
    width: 1em;
    height: 1em;
    border-radius: 3px;
    background: color-mix(in srgb, var(--g) 55%, #202022);
    border: 1px solid color-mix(in srgb, var(--g) 80%, #3a3a3e);
  }
  .lk2i {
    background: #151517;
    border: 1px dashed #4c4c53;
  }
</style>
