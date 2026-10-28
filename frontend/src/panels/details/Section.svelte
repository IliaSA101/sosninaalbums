<script lang="ts">
  import type { Snippet } from 'svelte'
  import { modes, type SectionName } from '../../base/prefs/view.svelte'

  let { name, title, children }: { name: SectionName; title: string; children: Snippet } = $props()

  const open = $derived(modes.sections[name])
</script>

<!-- Аккордеон: содержимое не размонтируется (форма настроек не теряет правки),
     высота анимируется через grid-template-rows 0fr → 1fr. -->
<section data-section={name}>
  <button class="head" aria-expanded={open} aria-controls="section-{name}" onclick={() => modes.toggleSection(name)}>
    <svg class="chev" class:open viewBox="0 0 12 12" aria-hidden="true"><path d="M4 2.5 7.5 6 4 9.5" /></svg>
    {title}
  </button>
  <div class="body" class:open id="section-{name}" inert={!open}>
    <div class="inner">
      <div class="pad">{@render children()}</div>
    </div>
  </div>
</section>

<style>
  section {
    border-top: 1px solid var(--line);
  }
  .head {
    all: unset;
    box-sizing: border-box;
    width: 100%;
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 12px 16px;
    font-weight: 600;
    cursor: pointer;
  }
  .head:hover {
    background: rgba(255, 255, 255, 0.04);
  }
  .head:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: -2px;
  }
  .chev {
    width: 12px;
    height: 12px;
    flex: none;
    fill: none;
    stroke: var(--muted);
    stroke-width: 1.6;
    stroke-linecap: round;
    stroke-linejoin: round;
    transition: transform var(--move);
  }
  .chev.open {
    transform: rotate(90deg);
  }
  .body {
    display: grid;
    grid-template-rows: 0fr;
    transition: grid-template-rows var(--move);
  }
  .body.open {
    grid-template-rows: 1fr;
  }
  .inner {
    min-height: 0;
    overflow: hidden;
  }
  .pad {
    padding: 0 16px 16px;
  }
</style>
