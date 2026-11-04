<script lang="ts">
  import { COVER_CHIP, REPEAT_MARK, roleTitle } from './looks'
  import { looks } from './looks.svelte'

  // Лента шагов под листом просмотра: «Обложка · 1 Титул · 2 Сборы ↻». Щелчок по шагу показывает его (131, 163 U3).
  const chips = $derived(looks.chips)
</script>

<div class="strip" role="group" aria-label="Шаги макета">
  {#each chips as c, k (c.i)}
    {#if k > 0}<span class="dot" aria-hidden="true">·</span>{/if}
    <button
      class="chip"
      class:on={c.i === looks.step}
      aria-pressed={c.i === looks.step}
      title={roleTitle(c.role) || (c.i === 0 ? COVER_CHIP : c.label)}
      onclick={() => (looks.step = c.i)}
      >{#if c.num > 0}<b>{c.num}</b>{/if}<span>{c.label}</span>{#if c.role === 'repeat'}<span class="rep">{REPEAT_MARK}</span>{/if}</button
    >
  {/each}
</div>

<style>
  .strip {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    justify-content: center;
    gap: 2px 3px;
    max-height: 64px;
    overflow-y: auto;
    padding: 6px 0;
    font-size: 12px;
  }
  .chip {
    display: inline-flex; /* номер, имя и ↻ — через gap, пробелов в разметке нет */
    align-items: center;
    gap: 4px;
    height: 22px;
    padding: 0 7px;
    font-size: 12px;
    background: transparent;
    color: var(--muted);
  }
  .chip b {
    font-weight: 600;
  }
  .chip.on {
    background: var(--accent);
    color: #1e1e1e;
  }
  .rep {
    opacity: 0.8;
  }
  .dot {
    color: var(--muted);
  }
</style>
