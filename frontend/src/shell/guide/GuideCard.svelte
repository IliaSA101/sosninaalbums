<script lang="ts">
  import { store } from '../../base/core/store.svelte'
  import { prefs } from '../../base/prefs/prefs.svelte'
  import { keymap } from '../../base/keys/keymap.svelte'
  import { shortOf } from '../../base/prefs/levelcustom'
  import { LEVEL_TITLE, minLevel, rowOf } from '../../base/prefs/levels'
  import type { Article } from './articles'
  import { guide } from './guidestate.svelte'
  import { closedLine, closedPlaces, keyRows, levelNeed, menuPath, openPlaces, planShow } from './show'

  // Карточка функции (спека 033): что делает, где, клавиши, уровень и «Показать». Тексты — только текстом.
  let { a }: { a: Article } = $props()

  const ctx = $derived(guide.ctx())
  const plan = $derived(planShow(a, ctx))
  const need = $derived(levelNeed(a, ctx))
  const hidden = $derived(need.steps.length > 0)
  const menus = $derived((a.menu ?? []).map(menuPath).filter((p): p is string => !!p))
  const keys = $derived(keyRows(a))
  // Кнопки — места, доступные на уровне; места выше уровня — серой строкой без кнопки (решение 133).
  const places = $derived(openPlaces(a, ctx))
  const later = $derived(closedPlaces(a, ctx))
  const minTitle = $derived(LEVEL_TITLE[minLevel(a.feature)])

  // Строка про уровень: готовый уровень называет нужный, «Свой» — что добавится.
  const levelLine = $derived.by(() => {
    if (!hidden) return `Уровень: ${minTitle}`
    if (prefs.level === 'custom') return `Скрыто в вашем наборе. «Показать» добавит: ${need.added.map(shortOf).join(', ')}`
    return `Скрыто на уровне «${LEVEL_TITLE[prefs.level]}». «Показать» включит «${LEVEL_TITLE[need.to ?? 'standard']}»`
  })

  // Только эта функция — в «Свой» (решение 84), без смены готового уровня целиком.
  function onlyThis() {
    const e = prefs.toggle(rowOf(a.feature), true)
    if (e.added.length === 0) return
    store.notify(`Включено в «Свой»: ${e.added.map(shortOf).join(', ')}`, 'info', { label: 'Вернуть', run: () => void prefs.apply(e.prev) })
  }
</script>

<article class="card" aria-label={a.title}>
  <h2>{a.title}</h2>
  <p class="what">{keymap.text(a.what)}</p>
  {#if a.where}<div class="row"><b>Где</b><span>{keymap.text(a.where)}</span></div>{/if}
  {#each menus as m (m)}<div class="row"><b>Меню</b><span>{m}</span></div>{/each}
  {#if a.how}<div class="row"><b>Как</b><span>{keymap.text(a.how)}</span></div>{/if}
  {#if keys.length > 0}
    <div class="row">
      <b>Клавиши</b>
      <span class="keys">
        {#each keys as k (k.show)}<span><kbd>{k.show}</kbd> — {k.label}</span>{/each}
      </span>
    </div>
  {/if}
  <div class="row" class:warn={hidden}>
    <b>Уровень</b>
    <span>
      {levelLine}
      {#if hidden && prefs.level !== 'custom'}
        <button class="link" onclick={onlyThis}>Только эту функцию — в «Свой»</button>
      {/if}
    </span>
  </div>
  {#if plan.note}<p class="note">{plan.note}</p>{/if}
  {#if places.length > 0}
    <div class="act">
      {#each places as p, i (p.spot)}
        <button class:accent={i === 0} onclick={() => guide.show(a.id, p.spot)}>{p.label}</button>
      {/each}
    </div>
  {/if}
  {#each later as p (p.spot)}<p class="later">{closedLine(p, ctx)}</p>{/each}
</article>

<style>
  .card {
    display: flex;
    flex-direction: column;
    gap: 8px;
    line-height: 1.4;
  }
  h2 {
    margin: 0;
    font-size: 15px;
    font-weight: 600;
  }
  .what {
    margin: 0;
  }
  .row {
    display: grid;
    grid-template-columns: 72px 1fr;
    gap: 8px;
    color: var(--muted);
  }
  .row b {
    font-weight: 600;
    color: var(--text);
  }
  .row.warn span {
    color: var(--accent);
  }
  .keys {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }
  kbd {
    font: inherit;
    font-weight: 600;
    color: var(--text);
  }
  .note {
    margin: 0;
    color: var(--muted);
    font-style: italic;
  }
  .act {
    margin-top: 4px;
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
  }
  .later {
    margin: 0;
    color: var(--muted);
  }
  .link {
    height: auto;
    padding: 0;
    margin-left: 6px;
    background: transparent;
    color: var(--accent);
    white-space: normal;
    text-align: left;
  }
</style>
