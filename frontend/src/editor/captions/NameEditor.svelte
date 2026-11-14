<script lang="ts">
  import type { SpreadView } from '../../base/core/types'
  import { store } from '../../base/core/store.svelte'
  import { edit } from '../text/edit.svelte'
  import PersonFields from '../../panels/names/PersonFields.svelte'
  import { onDestroy } from 'svelte'
  import { nameEdit } from './nameedit.svelte'
  import { namePlace, NAME_W } from './nametarget'

  // Окошко «Имя» под подписью (спека 037): поля имени шаблона по порядку, Enter — следующее, Esc или щелчок
  // мимо — закрыть. Поля — те же, что «Имя на фото» в карточке (одна сессия набора — один шаг отмены).
  // «Свой текст вместо шаблона» — прежняя правка подписи.
  let { spread, width }: { spread: SpreadView; width: number } = $props()

  const cur = $derived(nameEdit.cur?.spread === spread.id ? nameEdit.cur : null)
  const ppm = $derived(width / spread.w)
  const height = $derived((width * spread.h) / spread.w)
  let boxH = $state(0) // фактическая высота плашки (bind:clientHeight): строка «из имени файла» её меняет
  const rows = $derived(cur?.who.length ?? 0)
  // До первого замера — оценка по числу полей: поле 28 + зазор 6, ссылка «Свой текст» и отступы.
  const estH = $derived(rows * 34 + 46 + 16)
  // Не уже подписи, не уже NAME_W и не шире разворота (мини-разворот).
  const boxW = $derived(cur ? Math.min(width, Math.max(NAME_W, cur.box.w * ppm)) : NAME_W)
  const place = $derived(cur ? namePlace(cur.box, ppm, width, height, boxH || estH, boxW) : null)

  let box = $state<HTMLElement>()

  // Цель окошка (фото или пустое место) — последняя непустая: при закрытии cur уже null, а поля (blur,
  // досылка набранного по Esc) читают её ещё раз — ленивое cur.photo там бросало TypeError.
  let last = { photo: '', place: '', who: [] as readonly string[] }
  const snap = $derived.by(() => {
    if (cur) last = { photo: cur.photo, place: cur.place, who: cur.who }
    return last
  })
  const photo = $derived(snap.photo)
  // Ссылка на место — от примитивов: новый вид разворота (spread) каждую правку её не пересоздаёт.
  const sid = $derived(spread.id)
  const placeRef = $derived(snap.place ? { spread: sid, id: snap.place } : undefined)

  // Другой разворот или полей имени у фото больше нет — закрыть сразу.
  $effect(() => {
    if (nameEdit.verdict(spread, store.spread?.id) === 'close') nameEdit.close()
  })

  // Смена разворота при живом экземпляре. Без cleanup: новый вид с тем же id (store.apply) эффект
  // перезапускает, а сброс не должен закрывать окошко.
  $effect(() => nameEdit.sync(store.spread?.id))

  // Разворот ушёл из виду: Spread стал mini и NameEditor размонтирован — закрываем здесь, иначе при
  // возврате окошко всплывёт само.
  onDestroy(() => nameEdit.release(spread.id))

  function own() {
    const photo = cur?.photo
    nameEdit.close()
    if (photo) void edit.editCaption(photo)
  }
</script>

<svelte:window
  onpointerdowncapture={(e) => {
    if (cur && !box?.contains(e.target as Node)) nameEdit.close()
  }}
/>

{#if cur && place}
  <!-- Клики и клавиши окошка не уходят в разворот: не снимают выделение, не «вписывают», Esc закрывает только его -->
  <div
    bind:this={box}
    bind:clientHeight={boxH}
    class="name"
    style:left="{place.left}px"
    style:top="{place.top}px"
    style:width="{boxW}px"
    role="dialog"
    tabindex="-1"
    aria-label="Имя"
    data-chrome
    data-text-ui
    onpointerdown={(e) => e.stopPropagation()}
    onclick={(e) => e.stopPropagation()}
    ondblclick={(e) => e.stopPropagation()}
    onkeydown={(e) => {
      e.stopPropagation()
      if (e.key === 'Escape') nameEdit.close()
    }}
  >
    {#key snap.place || photo}
      <PersonFields {photo} place={placeRef} only={snap.who} inline ondone={() => nameEdit.close()} />
    {/key}
    <!-- «Свой текст» — правка подписи фото; у пустого места подписи нет -->
    {#if photo}<button type="button" class="own" onclick={own}>Свой текст вместо шаблона</button>{/if}
  </div>
{/if}

<style>
  /* Плашка как у Popover: одна рамка, тень, отступ 10 px; без outline (фокус на самой плашке рисовал цветную рамку) */
  .name {
    position: absolute;
    transform: translateX(-50%);
    z-index: 9;
    box-sizing: border-box;
    padding: 10px;
    display: grid;
    grid-template-columns: minmax(0, 1fr); /* иначе поля с size=20 растягивают колонку шире плашки */
    gap: 6px;
    background: var(--panel);
    border: 1px solid var(--line);
    border-radius: 8px;
    box-shadow: 0 10px 32px rgba(0, 0, 0, 0.5);
    outline: none;
    cursor: default;
  }
  .own {
    justify-self: start;
    height: auto;
    padding: 2px 0 0;
    font-size: 11px;
    line-height: 14px;
    white-space: normal;
    text-align: left;
    color: var(--muted);
    background: transparent;
    text-decoration: underline;
  }
  .own:hover:not(:disabled) {
    color: #fff;
    background: transparent;
  }
</style>
