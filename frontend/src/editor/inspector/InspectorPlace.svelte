<script lang="ts">
  import { placesel } from '../places/placesel.svelte'
  import { groupTitle } from './panels'
  import { prefs } from '../../base/prefs/prefs.svelte'
  import PlaceFields from './PlaceFields.svelte'
  import GroupFields from './GroupFields.svelte'
  import FitToggle from './FitToggle.svelte'
  import SnapToggle from './SnapToggle.svelte'

  // Карточка пустого места или группы (спеки 008, 014). Место: X/Y/Ш/В образца, замок, привязка (общая с текстом),
  // «Целиком / С обрезкой». Группа (2+ объекта, в том числе тексты): состав в заголовке и X/Y/Ш/В общей рамки.
  // Удалить — в ленте и по Delete.
  const title = $derived(placesel.isGroup ? groupTitle(placesel.selected.length, placesel.texts.length) : 'Место')
</script>

<div class="head">{title}</div>
{#if prefs.can('manual')}
  {#if placesel.isGroup}<GroupFields />{:else}<PlaceFields />{/if}
  <SnapToggle />
{/if}
{#if prefs.can('frame') && placesel.selected.length > 0}<FitToggle />{/if}

<style>
  .head {
    font-size: 13px;
    font-weight: 600;
    margin-bottom: 8px;
  }
</style>
