<script lang="ts">
  import { store } from '../../base/core/store.svelte'
  import { errText } from '../../base/words/errtext'
  import { sheetSize } from '../../base/geom/sheet'
  import type { SpreadView, VigChoice, VignetteView } from '../../base/core/types'
  import VigCard from './VigCard.svelte'
  import { gallery } from '../gallery/gallery.svelte'
  import { sortVignette, vigChoices } from './vigact'
  import { emptyPlaces, fitBox, summaryText } from './vigdialog'
  import { tplLabel } from './vigown'
  import { vigOwn } from './vigown.svelte'
  import { vigDialog } from './vigsession.svelte'
  import { vigFaces } from './vigfaces.svelte'
  import { facesLabel, facesTip } from './vigfaces'
  import { choiceOf, formOf } from './vignette'
  import { VIG_SORT_TITLE } from './vigmenu'

  // Раздел «Виньетка» карточки справа — только сводка и кнопки (031 dialog §2, отзыв 14.10): схема шаблона,
  // имя, числа и размер портрета. Все параметры — в окне «Настроить виньетку» (VigDialog).
  // Схему и размер портрета считает Go (VignetteChoices); здесь перевод долей листа в проценты.
  // «Сохранить как шаблон…» и «Обычный разворот» — в «⋯» группы (vigmenu.ts); onsummary — строка группы в свёрнутом виде.
  let { sp, onsummary }: { sp: SpreadView & { vig: VignetteView }; onsummary?: (text: string) => void } = $props()

  const vig = $derived(sp.vig)
  const MAX_H = 120 // высота схемы не больше, px

  // Карточка текущего шаблона при числах листа; пока ответа нет — только текст.
  let items = $state<VigChoice[]>([])
  let req = 0
  $effect(() => {
    const f = formOf(vig)
    void store.view.rev
    void vigOwn.rev // свой шаблон переименовали или удалили
    const my = ++req
    vigChoices(sp.id, f)
      .then((list) => {
        if (my === req) items = list
      })
      .catch((e) => {
        if (my === req) store.notify(errText(e), 'error')
      })
  })
  const now = $derived(choiceOf(items, vig.tpl))
  const empty = $derived(emptyPlaces(sp))
  $effect(() => {
    onsummary?.(summaryText(vig, now))
  })

  // Схема на всю ширину карточки, но не выше MAX_H; пропорция листа та же.
  let boxW = $state(0)
  const dims = $derived(fitBox(sheetSize(sp).w, sheetSize(sp).h, boxW, MAX_H))
  const galleryOpen = $derived(gallery.cur?.spread === sp.id && gallery.cur.tab === 'layouts')
  const setup = () => vigDialog.open(sp.id)
  // Подгон по лицам (130): пока идёт поиск, кнопка aria-disabled и показывает ход; Go ищет по одному листу за раз.
  const faces = $derived(vigFaces.progress(sp.id))
</script>

<div class="vp" role="group" aria-label="Виньетка">
  <div class="scheme" bind:clientWidth={boxW}>
    {#if now && dims.w > 0}
      <button type="button" class="pic" title="Настроить виньетку…" aria-label="Настроить виньетку" onclick={setup}>
        <VigCard c={now} w={dims.w} h={dims.h} form={formOf(vig)} />
      </button>
    {/if}
  </div>
  <div class="tname">{tplLabel(items, vig.tpl, vig.tplName)}</div>
  <div class="sum" class:small={now?.small}>{summaryText(vig, now)}</div>
  {#if empty > 0}<div class="sum">Пустых мест: {empty}</div>{/if}

  <button type="button" class="primary setup" data-guide="vig-setup" onclick={setup}>Настроить виньетку…</button>
  <div class="tools">
    <button
      type="button"
      data-guide="vig-templates"
      aria-pressed={galleryOpen}
      title="Шаблоны рядом над листом, как раскладки (⊞ в полосе над листом, пробел — следующий). Шаблон встаёт с числами, что уже на листе"
      onclick={() => gallery.open('spread')}>Шаблоны…</button
    >
    <button type="button" data-guide="vig-sort" onclick={() => sortVignette(sp.id)} title={VIG_SORT_TITLE}>Сортировать по фамилии</button>
    <button
      type="button"
      class="faces"
      data-guide="vig-faces"
      aria-disabled={vigFaces.busy}
      onclick={() => vigFaces.fit(sp.id)}
      title={facesTip()}>{facesLabel(faces)}</button
    >
  </div>
</div>

<style>
  .vp {
    display: flex;
    flex-direction: column;
    gap: 4px;
    padding: 0 0 4px;
  }
  .scheme {
    display: flex;
    justify-content: center;
    min-width: 0;
    min-height: 8px;
  }
  /* Схема кликабельна: щелчок открывает окно, как кнопка ниже. Высоту глобальной button (30px) сбрасываем:
     иначе кнопка короче схемы, и схема вылезает на текст сводки. */
  .pic,
  .pic:hover:not(:disabled) {
    display: block;
    height: auto;
    max-width: 100%;
    padding: 0;
    background: transparent;
    border: 0;
    border-radius: 0;
    line-height: 0;
    cursor: pointer;
  }
  .pic:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: 2px;
  }
  .tname {
    margin-top: 4px;
    font-size: 13px;
    overflow-wrap: anywhere;
  }
  .sum {
    overflow-wrap: anywhere;
    font-size: 12px;
    line-height: 15px;
    color: var(--muted);
  }
  .sum.small {
    color: var(--accent);
  }
  .setup {
    height: 30px;
    margin-top: 10px;
  }
  .tools {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
  }
  .tools button {
    flex: 1 1 auto;
    min-width: 0;
    height: 26px;
    padding: 0 6px;
    overflow: hidden;
    text-overflow: ellipsis;
    font-size: 12px;
  }
  /* Подпись «Лица 12 / 25» короче «Подогнать по лицам»: ширина по длинной, чтобы ряд не прыгал (как «Экспорт 12 / 40») */
  .tools button.faces {
    min-width: 17ch;
  }
  .tools button[aria-disabled='true'] {
    opacity: 0.55;
    cursor: default;
  }
  .tools button[aria-pressed='true'] {
    border-color: var(--accent);
  }
</style>
