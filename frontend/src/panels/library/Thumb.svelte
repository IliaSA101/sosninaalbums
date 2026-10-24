<script lang="ts">
  import { store } from '../../base/core/store.svelte'
  import { drag } from '../../base/drag/drag.svelte'
  import type { Photo } from '../../base/core/types'
  import Icon from '../../base/ui/Icon.svelte'
  import MissPhoto from '../../base/ui/MissPhoto.svelte'
  import { thumbUrl } from '../../base/core/media'
  import { cancelOnUnmount } from '../../base/ui/imgcancel'
  import { colorBadge } from '../../editor/photo/color'
  import { markOf } from '../marks/marks'
  import { markTitle, stars } from '../marks/thumbmark'
  import { binsel } from '../bins/binsel.svelte'
  import { lost } from './lost.svelte'
  import { missTip } from './missing'
  import { thumbmenu } from './thumbmenustate.svelte'

  // Миниатюра фото в библиотеке. Размер задаёт --size родителя (.grid в Library).
  let { p }: { p: Photo } = $props()
  const badge = $derived(colorBadge(p))
  // Действующая отметка (своя по полю + XMP из файла): полоса, звёзды, флажок, подсказка
  const mark = $derived(markOf(p, store.marks[p.id]))
  const rejected = $derived(mark.flag === -1)
  // Сколько раз фото в альбоме (098-6, решение 0.23-q8: один раз — ✓, больше — число): места всех копий, фоны, стикеры, рамочки, фон альбома — то же, что «Использовано N раз».
  const uses = $derived(store.useCount(p.id))
</script>

<div
  class="thumb"
  data-thumb={p.id}
  class:used={store.used.has(p.id)}
  class:selected={store.selected.has(p.id)}
  class:rejected
  role="button"
  tabindex="-1"
  title={markTitle(p.name, mark, binsel.namesOf(store.marks[p.id]?.bins))}
  onpointerdown={(e) =>
    drag.start(
      e,
      () => ({ kind: 'photos', ids: store.selected.has(p.id) ? store.selectedIds : [p.id] }),
      () => store.dragIds(p.id), // выделение меняется, только когда перенос начался
    )}
  onclick={(e) => drag.clickAllowed() && store.click(p.id, e)}
  ondblclick={() => store.addToCurrent([p.id])}
  oncontextmenu={(e) => thumbmenu.open(e, p.id)}
  onkeydown={() => {}}
>
  <!-- .im повторяет форму фото и вписан в квадратную ячейку: рамка идёт по самому фото -->
  <div class="im" class:tall={p.h > p.w} style:aspect-ratio="{p.w} / {p.h}">
    {#if lost.has(p.id)}
      <!-- Файл пропал до проверки папок (088): заглушка вместо битой картинки, рамка — как у img -->
      <div class="gone"><MissPhoto size={24} tip={missTip(p.id, { [p.id]: p.name })} label="нет файла" /></div>
    {:else}
      <!-- retryOf: разовая перезагрузка после сверки папок, если файл на месте (lost.retry) -->
      <img
        src={thumbUrl(p.id, lost.retryOf(p.id))}
        alt=""
        loading="lazy"
        draggable="false"
        onerror={() => lost.mark(p.id)}
        {@attach cancelOnUnmount}
      />
    {/if}
    {#if uses > 0}<span class="check" aria-label={uses === 1 ? 'Уже в альбоме' : `В альбоме: ${uses}`}>{uses === 1 ? '✓' : uses}</span>{/if}
    {#if badge}<span class="badge {badge.level}" data-guide="color-badge" title={badge.title}>{badge.label}</span>{/if}
    {#if mark.label !== ''}<span class="strip {mark.label}"></span>{/if}
    {#if mark.rating > 0}<span class="stars">{stars(mark.rating)}</span>{/if}
    {#if mark.flag === 1}<span class="flag"><Icon name="flag" size={14} /></span>{:else if rejected}<span class="flag"
        ><Icon name="flag-off" size={14} /></span
      >{/if}
  </div>
</div>

<style>
  /* Ячейка одинаковая у всех: размер задаёт --size, имя файла — в подсказке */
  .thumb {
    width: var(--size);
    height: var(--size);
    display: grid;
    place-items: center;
    cursor: grab;
    overflow: hidden;
    flex: none;
    container-type: size; /* для @container у звёзд: размер ячейки задан явно */
  }
  /* Размер — от --size, а не в процентах: у элемента сетки с place-items проценты высоты
     не считаются, и вертикальные фото вылезали на соседний ряд */
  .im {
    position: relative;
    width: var(--size);
  }
  .im.tall {
    width: auto;
    height: var(--size);
  }
  .thumb img {
    display: block;
    width: 100%;
    height: 100%;
    transition: opacity 0.2s;
    object-fit: contain;
    background: #333;
    border: 2px solid #fff;
    box-sizing: border-box;
    user-select: none;
  }
  .thumb.selected img {
    border-color: var(--accent);
  }
  /* Заглушка «нет файла» в форме фото: белая рамка 2 px, как у img; бледность used/rejected к ней не применяется */
  .gone {
    position: absolute;
    inset: 0;
    border: 2px solid #fff;
    box-sizing: border-box;
    overflow: hidden;
  }
  .thumb.selected .gone {
    border-color: var(--accent);
  }
  /* Уже в альбоме: число в углу и чуть бледнее. Одной бледности мало — на ярких
     и блёклых снимках её не отличить от самого фото (отзыв пользователя 2026-10-08) */
  .thumb.used img {
    opacity: 0.75;
    filter: saturate(0.4); /* «серее»: бледность одна не отличает от блёклого снимка */
  }
  .thumb.used.selected img {
    opacity: 0.9;
    filter: saturate(0.6);
  }
  /* Отклонённое: серее и бледнее, чем «в альбоме» (там 0.75), и после .used — перекрывает его */
  .thumb.rejected img {
    opacity: 0.45;
    filter: grayscale(1);
  }
  .thumb.rejected.selected img {
    opacity: 0.65;
  }
  /* Оверлеи отметок не ловят мышь: перенос, щелчок и двойной щелчок идут в .thumb */
  .strip,
  .stars,
  .flag {
    position: absolute;
    pointer-events: none;
  }
  /* Полоса цвета метки по нижнему краю фото, внутри белой рамки */
  .strip {
    left: 2px;
    right: 2px;
    bottom: 2px;
    height: 4px;
    background: var(--lbl-other);
  }
  .strip.red {
    background: var(--lbl-red);
  }
  .strip.yellow {
    background: var(--lbl-yellow);
  }
  .strip.green {
    background: var(--lbl-green);
  }
  .strip.blue {
    background: var(--lbl-blue);
  }
  .strip.purple {
    background: var(--lbl-purple);
  }
  /* Звёзды справа внизу: белым с тенью, читаются на светлом и тёмном */
  .stars {
    right: 6px;
    bottom: 8px;
    font-size: 11px;
    line-height: 14px;
    color: #fff;
    text-shadow:
      0 0 2px rgb(0 0 0 / 0.9),
      0 1px 2px rgb(0 0 0 / 0.9);
  }
  /* Мелкие миниатюры (до ~110 px): пять звёзд занимают всю ширину и наехали бы на значок профиля
     внизу слева — поднимаем их над значком */
  @container (max-width: 110px) {
    .stars {
      bottom: 26px;
    }
  }
  /* Флажок слева вверху: флажок — отобрано, перечёркнутый — отклонено. Не кнопка: курсор
     и наведение — от самой миниатюры (pointer-events: none выше) */
  .flag {
    left: 5px;
    top: 5px;
    display: block;
    line-height: 0;
    color: #fff;
    filter: drop-shadow(0 0 1.5px rgb(0 0 0 / 0.9)) drop-shadow(0 1px 1px rgb(0 0 0 / 0.9));
  }
  /* ✓ при одном вхождении и число при нескольких — в одном кружке; трёхзначное растягивает кружок в капсулу */
  .check {
    position: absolute;
    top: 6px;
    right: 6px;
    min-width: 18px;
    height: 18px;
    padding: 0 4px;
    box-sizing: border-box;
    border-radius: 9px;
    background: rgb(0 0 0 / 0.6);
    color: #fff;
    font-size: 11px;
    line-height: 18px;
    text-align: center;
    pointer-events: none;
  }
  /* Значок цветового профиля: внизу слева */
  .badge {
    position: absolute;
    left: 6px;
    bottom: 6px;
    min-width: 16px;
    height: 16px;
    padding: 0 4px;
    box-sizing: border-box;
    border-radius: 8px;
    font-size: 10px;
    font-weight: 700;
    line-height: 16px;
    text-align: center;
    background: rgba(0, 0, 0, 0.65);
    color: #fff;
  }
  .badge.warn {
    background: #e0803a;
    color: #1e1e1e;
  }
</style>
