<script lang="ts">
  import { tick } from 'svelte'
  import Section from './Section.svelte'
  import AlbumInfo from './AlbumInfo.svelte'
  import SettingsForm from './SettingsForm.svelte'
  import SpreadGrid from './SpreadGrid.svelte'
  import Inspector from '../../editor/inspector/Inspector.svelte'
  import { store } from '../../base/core/store.svelte'
  import { modes } from '../../base/prefs/view.svelte'
  import { prefs } from '../../base/prefs/prefs.svelte'

  const s = $derived(store.view.settings)
  // Без лишних нулей: 21.5, а не 21.50.
  const num = (n: number) => String(Math.round(n * 100) / 100)

  let scroller: HTMLDivElement

  // Кнопка «Формат» в шапке: прокрутить к разделу, даже если панель уже открыта.
  $effect(() => {
    const r = modes.reveal
    if (!r) return
    tick().then(() => {
      const el = scroller?.querySelector<HTMLElement>(`[data-section="${r.name}"]`)
      if (el) scroller.scrollTo({ top: el.offsetTop, behavior: 'smooth' })
    })
  })
</script>

<!-- Ширина анимируется, содержимое фиксированной ширины 400 px: текст не перетекает. -->
<aside class="details" class:open={modes.details} inert={!modes.details} aria-label="Панель деталей">
  <!-- Карточка свойств выделенного: при открытой панели — здесь, сверху, вне прокрутки -->
  {#if modes.details}<div class="dock"><Inspector docked /></div>{/if}
  <div class="scroll" bind:this={scroller}>
    <div class="format">
      <div class="name">{num(s.pageW / 10)}×{num(s.pageH / 10)}</div>
      <div class="sub">разворот {num((s.pageW * 2) / 10)}×{num(s.pageH / 10)} см, {s.dpi} dpi</div>
    </div>
    <Section name="info" title="Сведения об альбоме"><AlbumInfo /></Section>
    <Section name="settings" title="Настройки альбома"><SettingsForm /></Section>
    {#if prefs.can('allSpreads')}<Section name="grid" title="Все развороты"><SpreadGrid /></Section>{/if}
  </div>
</aside>

<style>
  .details {
    flex: none;
    display: flex;
    flex-direction: column;
    width: 0;
    overflow: hidden;
    background: var(--panel);
    /* Линия слева тенью, а не рамкой: при ширине 0 рамка осталась бы видна */
    box-shadow: inset 1px 0 0 var(--line);
    transition: width var(--move);
  }
  .details.open {
    width: 400px;
  }
  .dock {
    flex: none;
    width: 400px;
    padding-left: 1px;
    display: flex;
    flex-direction: column;
  }
  .scroll {
    position: relative; /* offsetTop разделов — от начала прокрутки */
    width: 400px;
    flex: 1;
    min-height: 0;
    overflow-x: hidden;
    overflow-y: auto;
    padding-left: 1px;
  }
  .format {
    padding: 18px 16px 16px;
  }
  .name {
    font-size: 30px;
    font-weight: 600;
    line-height: 1.1;
  }
  .sub {
    margin-top: 4px;
    font-size: 13px;
    color: var(--muted);
  }
</style>
