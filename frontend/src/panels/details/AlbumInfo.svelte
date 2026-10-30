<script lang="ts">
  import { store } from '../../base/core/store.svelte'
  import { plural } from '../../base/words/plural'
  import { unitOf, isSheet } from '../../base/geom/sheet'
  import { albumStats, colorLine, limitLine, nextLowSheet } from './stats'

  const st = $derived(albumStats(store.sheets, store.byId, store.photos.length, store.lead))
  const color = $derived(colorLine(st))
  const unit = $derived(unitOf(store.view.settings))
  const sheet = $derived(isSheet(store.view.settings))
  const limit = $derived(limitLine(store.view))

  // Следующий лист с малым разрешением по кругу: развороты после текущего, обложка (032), снова с начала.
  function goLow() {
    const i = nextLowSheet(store.sheets, store.sheetIndex)
    if (i >= 0) store.goSheet(i)
  }
</script>

<ul>
  <li>
    {st.spreads}
    {plural(st.spreads, unit.plural)}
    {#if !sheet}({st.pages}
      {plural(st.pages, ['страница', 'страницы', 'страниц'])}){/if}
  </li>
  {#if limit}<li class:warn={limit.warn} class:muted={!limit.warn}>{limit.text}</li>{/if}
  <li>Уникальных раскладок: {st.layouts}</li>
  <li>Фото в альбоме: {st.photos} из {st.total}</li>
  <li class="muted">Горизонтальных: {st.h} · Вертикальных: {st.v} · Квадратных: {st.sq}</li>
  {#if color}<li>{color}</li>{/if}
  {#if store.view.emptyPlaces > 0}<li class="warn">Пустых мест: {store.view.emptyPlaces}</li>{/if}
  {#if store.view.lowDPI > 0}
    <li class="warn">
      <!-- Переход к следующему развороту с малым разрешением, по кругу -->
      <button type="button" class="link" onclick={() => goLow()}>
        Мало разрешения: {store.view.lowDPI} фото
      </button>
    </li>
  {/if}
</ul>

<style>
  ul {
    list-style: none;
    margin: 0;
    padding: 0;
    display: grid;
    gap: 6px;
    font-size: 13px;
    font-variant-numeric: tabular-nums;
  }
  .muted {
    color: var(--muted);
  }
  .warn {
    color: #e0806f;
  }
  .link {
    all: unset;
    cursor: pointer;
    text-decoration: underline dotted;
    text-underline-offset: 3px;
  }
  .link:hover,
  .link:focus-visible {
    text-decoration-style: solid;
  }
  .link:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: 2px;
  }
</style>
