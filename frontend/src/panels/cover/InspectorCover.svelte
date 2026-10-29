<script lang="ts">
  import { onMount } from 'svelte'
  import { store } from '../../base/core/store.svelte'
  import ColorPicker from '../../base/color/ColorPicker.svelte'
  import { colorLive } from '../../base/color/colorlive.svelte'
  import { isCover } from '../../base/geom/sheet'
  import CardGroup from '../../editor/inspector/CardGroup.svelte'
  import { spineSum } from '../../editor/inspector/groupsum'
  import SpreadBgGroup from '../../editor/inspector/SpreadBgGroup.svelte'
  import SpreadCapsGroup from '../../editor/inspector/SpreadCapsGroup.svelte'
  import { coverAct } from './coveract.svelte'
  import CoverLayoutGroup from './CoverLayoutGroup.svelte'
  import CoverSizeGroup from './CoverSizeGroup.svelte'
  import { overLine, spineAction } from './coverpanel'

  // Карточка обложки (спека 032, п. 9–13; 124 U6): предупреждения без группы (их нельзя свернуть), затем группы
  // «Фото фона» (схемы и зоны фото) → «Корешок» → «Фон» → «Подписи» → «Размеры для типографии».
  // Все числа — из View.cover; «Удалить обложку» — над листом и в меню плитки, в карточке нет (0.26-124-6).
  const cv = $derived(store.cover)
  const sp = $derived(isCover(store.spread) ? store.spread : null)
  const over = $derived(cv ? overLine(cv) : '')
  const spine = $derived(spineAction(cv))

  // Не $effect: он зависел от loading внутри loadPresets и при пустом ответе гонял запрос по кругу.
  onMount(() => void coverAct.loadPresets())
</script>

{#if cv}
  <div class="cover" role="group" aria-label="Обложка">
    {#if over}<div class="warn">{over}</div>{/if}
    {#if cv.missing}<div class="warn">{cv.missing}</div>{/if}
    {#if cv.fix}
      <div class="warn">{cv.fix}</div>
      <div class="btns">
        <button onclick={() => coverAct.applyPreset(cv.preset)}>Исправить</button>
      </div>
    {/if}

    {#if sp}<CoverLayoutGroup {sp} />{/if}

    <CardGroup id="cover.spine" title="Корешок" summary={spineSum({ text: !!cv.spineText, plate: !!cv.spineBg })}>
      <div class="spine" data-guide="cover-spine">
        <button
          class="spinebtn"
          title={spine.kind === 'add' ? 'Текст вдоль корешка по центру, на цветной плашке во весь корешок' : 'Выделить надпись на корешке'}
          onclick={() => coverAct.spineText()}>{spine.label}</button
        >
        <div class="row">
          <span class="plate">Плашка</span>
          <ColorPicker
            value={cv.spineBg || '#ffffff'}
            sheet={store.spread?.id}
            label="Цвет плашки корешка"
            onpick={(c) => colorLive.commit(() => coverAct.setSpineBg(c))}
            onpreview={(c) => (c ? colorLive.set(['spine'], c) : colorLive.clear())}
          />
          <button class="reset" disabled={!cv.spineBg} aria-label="Без плашки" title="Без плашки" onclick={() => coverAct.setSpineBg('')}
            >Без плашки</button
          >
        </div>
      </div>
    </CardGroup>

    {#if sp}
      <SpreadBgGroup {sp} />
      <SpreadCapsGroup {sp} />
    {/if}

    <CoverSizeGroup />
  </div>
{/if}

<style>
  .cover {
    display: flex;
    flex-direction: column;
    font-size: 12px;
  }
  .warn {
    color: #e0806f;
  }
  .warn + .warn,
  .warn + .btns {
    margin-top: 4px;
  }
  .spine {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }
  .row {
    display: flex;
    align-items: center;
    gap: 6px;
  }
  .spinebtn {
    height: 24px;
    padding: 0 8px;
    font-size: 12px;
  }
  .plate {
    flex: 1;
    min-width: 40px;
    color: var(--muted);
  }
  .reset {
    height: 24px;
    padding: 0 6px;
    font-size: 12px;
    background: transparent;
    color: #cfcfd4;
    border-radius: 4px;
  }
  .btns {
    display: flex;
    flex-wrap: wrap; /* узкая колонка: кнопки переносятся, а не сжимаются */
    gap: 6px;
  }
  .btns button {
    flex: 1 0 auto;
    white-space: nowrap;
    height: 24px;
    padding: 0 6px;
    font-size: 12px;
  }
</style>
