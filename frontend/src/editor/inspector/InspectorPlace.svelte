<script lang="ts">
  import { placesel } from '../places/placesel.svelte'
  import { groupTitle, seatOf } from './panels'
  import { store } from '../../base/core/store.svelte'
  import PersonFields from '../../panels/names/PersonFields.svelte'
  import { prefs } from '../../base/prefs/prefs.svelte'
  import PlaceFields from './PlaceFields.svelte'
  import GroupFields from './GroupFields.svelte'
  import FitToggle from '../photoprops/FitToggle.svelte'
  import ShapeToggle from '../photoprops/ShapeToggle.svelte'
  import SnapToggle from '../photoprops/SnapToggle.svelte'

  // Карточка пустого места или группы (спеки 008, 014). Место: X/Y/Ш/В образца, замок, привязка (общая с текстом),
  // «Целиком / С обрезкой». Группа (2+ объекта, в том числе тексты): состав в заголовке и X/Y/Ш/В общей рамки.
  // Один пустой выделенный — ещё «Имя на месте» (071): табличка, которая ждёт фото. Удалить — в ленте и по Delete.
  // head=false — заголовок рисует сама карточка (docked, cardTitle): сворачиваемая шапка.
  let { head = true }: { head?: boolean } = $props()
  const title = $derived(
    placesel.isGroup ? groupTitle(placesel.selected.length, placesel.texts.length, placesel.selectedStickers.length) : 'Место',
  )
  const seat = $derived(seatOf(placesel.isGroup, placesel.selected))
  // Ссылка на место — от примитивов: новый вид разворота каждую правку её не пересоздаёт (иначе поля перезапускают загрузку).
  const sid = $derived(store.spread?.id ?? '')
  const seatRef = $derived({ spread: sid, id: seat })
</script>

{#if head}<div class="head">{title}</div>{/if}
{#if prefs.can('manual')}
  {#if placesel.isGroup}<GroupFields />{:else}<PlaceFields />{/if}
  <SnapToggle />
{/if}
{#if prefs.can('frame') && placesel.selected.length > 0}<FitToggle /><ShapeToggle />{/if}
{#if seat && sid && prefs.can('captions')}
  <!-- По месту пересоздаётся: набранное уходит на своё место -->
  {#key seat}<PersonFields photo="" place={seatRef} />{/key}
{/if}

<style>
  .head {
    font-size: 13px;
    font-weight: 600;
    margin-bottom: 8px;
  }
</style>
