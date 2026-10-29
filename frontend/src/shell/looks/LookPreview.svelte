<script lang="ts">
  import { screenDpr } from '../../base/ui/dpr.svelte'
  import { store } from '../../base/core/store.svelte'
  import { lookUrl } from '../../base/core/media'
  import { tilePx } from '../../panels/decor/decor'
  import { COVER_CHIP, PICK_HINT, REPEAT_MARK, lookKey, lookRatio, roleTitle } from './looks'
  import { looks } from './looks.svelte'

  // Предпросмотр справа: крупный лист шага, ‹ › (и ←/→ в окне) и лента шагов «Обложка · 1 Титул · 2 Сборы ↻».
  // Режим «Фоны по серии» показывает только фон шага (bg=1). Картинку рисует Go по пропорции формата альбома.
  const W = 340 // CSS px
  const px = $derived(Math.min(1024, tilePx(W, screenDpr.value)))
  const chips = $derived(looks.chips)
  const at = $derived(chips.findIndex((c) => c.i === looks.step))
  const stepInfo = $derived(looks.step > 0 ? looks.info?.steps[looks.step - 1] : undefined)
  const r = $derived(lookRatio(store.view.settings, stepInfo?.pages ?? 2, looks.step === 0 ? store.cover?.sheet : null))
  const src = $derived(looks.sel ? lookUrl(lookKey(looks.sel), { i: looks.step, px, r, bg: looks.mode === 'bg' }) : '')
</script>

<div class="pv">
  {#if !looks.sel}
    <div class="hint">{PICK_HINT}</div>
  {:else}
    <div class="stage">
      <button class="nav" disabled={at <= 0} aria-label="Предыдущий шаг" title="Предыдущий шаг" onclick={() => looks.go(-1)}>‹</button>
      <div class="sheet" style:aspect-ratio={r}>
        <img src={src || undefined} alt="" draggable="false" />
      </div>
      <button
        class="nav"
        disabled={at < 0 || at >= chips.length - 1}
        aria-label="Следующий шаг"
        title="Следующий шаг"
        onclick={() => looks.go(1)}>›</button
      >
    </div>
    <div class="strip" role="group" aria-label="Шаги макета">
      {#each chips as c, k (c.i)}
        {#if k > 0}<span class="dot" aria-hidden="true">·</span>{/if}
        <button
          class="chip"
          class:on={c.i === looks.step}
          aria-pressed={c.i === looks.step}
          title={roleTitle(c.role) || (c.i === 0 ? COVER_CHIP : c.label)}
          onclick={() => (looks.step = c.i)}
          >{#if c.num > 0}<b>{c.num}</b>{/if}<span>{c.label}</span>{#if c.role === 'repeat'}<span class="rep">{REPEAT_MARK}</span
            >{/if}</button
        >
      {/each}
    </div>
  {/if}
</div>

<style>
  .pv {
    flex: none;
    min-height: 0;
    display: flex;
    flex-direction: column;
    gap: 8px;
  }
  .hint {
    padding: 30px 0 0 8px;
    color: var(--muted);
    font-size: 13px;
  }
  .stage {
    display: flex;
    align-items: center;
    gap: 4px;
  }
  .nav {
    flex: none;
    width: 22px;
    padding: 0;
    font-size: 18px;
    line-height: 1;
  }
  .sheet {
    flex: 1;
    min-width: 0;
    max-height: 300px;
    margin: 0 auto;
    position: relative;
    overflow: hidden;
    border: 1px solid var(--line);
    border-radius: 4px;
    background: var(--panel2);
  }
  .sheet img {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
    object-fit: contain;
  }
  .strip {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 2px 3px;
    max-height: 64px;
    overflow-y: auto;
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
