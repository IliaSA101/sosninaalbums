<script lang="ts">
  import { prefs } from '../../base/prefs/prefs.svelte'
  import { ALL_LEVELS, LEVEL_POINTS, LEVEL_TITLE, customPoints, type Level } from '../../base/prefs/levels'

  // Четыре карточки уровней в ряд (спека 011, п. 1): щелчок сразу включает уровень, текущий обведён.
  let { onpick }: { onpick: (l: Level) => void } = $props()

  // Основа «Своего» для строки «Основа сейчас»: набор, а если его не было — текущий готовый уровень.
  const base = $derived(prefs.custom?.base ?? (prefs.level === 'custom' ? 'simple' : prefs.level))
  const points = (l: Level) => (l === 'custom' ? customPoints(base) : LEVEL_POINTS[l])
</script>

<div class="levels" role="radiogroup" aria-label="Уровень интерфейса">
  {#each ALL_LEVELS as l (l)}
    <button class="lvl" role="radio" aria-checked={prefs.level === l} onclick={() => onpick(l)}>
      <b>{LEVEL_TITLE[l]}</b>
      <ul>
        {#each points(l) as p (p)}
          <li>{p}</li>
        {/each}
      </ul>
    </button>
  {/each}
</div>

<style>
  .levels {
    display: grid;
    grid-template-columns: repeat(4, 1fr);
    gap: 8px;
  }
  .lvl {
    height: auto;
    padding: 8px 10px;
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    gap: 4px;
    text-align: left;
    background: var(--panel2);
    border: 1px solid var(--line);
    border-radius: 6px;
  }
  .lvl:hover {
    border-color: var(--muted);
  }
  .lvl[aria-checked='true'] {
    border-color: var(--accent);
  }
  ul {
    margin: 0;
    padding: 0;
    list-style: none;
    font-size: 11px;
    line-height: 16px;
    color: var(--muted);
  }
</style>
