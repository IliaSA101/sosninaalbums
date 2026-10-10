<script lang="ts">
  import { store } from '../../base/core/store.svelte'
  import { plural } from '../../base/core/plural'
  import { albumStats, colorLine } from './stats'

  const st = $derived(albumStats(store.view.spreads, store.byId, store.photos.length))
  const color = $derived(colorLine(st))
</script>

<ul>
  <li>
    {st.spreads}
    {plural(st.spreads, ['разворот', 'разворота', 'разворотов'])} ({st.pages}
    {plural(st.pages, ['страница', 'страницы', 'страниц'])})
  </li>
  <li>Уникальных раскладок: {st.layouts}</li>
  <li>Фото в альбоме: {st.photos} из {st.total}</li>
  <li class="muted">Горизонтальных: {st.h} · Вертикальных: {st.v} · Квадратных: {st.sq}</li>
  {#if color}<li>{color}</li>{/if}
  {#if store.view.emptyPlaces > 0}<li class="warn">Пустых мест: {store.view.emptyPlaces}</li>{/if}
  {#if store.view.lowDPI > 0}<li class="warn">Мало точек: {store.view.lowDPI} фото</li>{/if}
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
</style>
