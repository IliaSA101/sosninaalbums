<script lang="ts">
  import { edit } from './edit.svelte'
  import { store } from '../../base/core/store.svelte'
  import { placesel } from '../places/placesel.svelte'
  import { styleSupport } from './fonts'
  import { MAX_SIZE, MIN_SIZE, clampSize } from './textedit'
  import ColorPicker from '../../base/ui/ColorPicker.svelte'
  import Icon from '../../base/ui/Icon.svelte'
  import { prefs } from '../../base/prefs/prefs.svelte'
  import SnapToggle from '../photoprops/SnapToggle.svelte'
  import LayerRow from '../inspector/LayerRow.svelte'
  import TextGeomFields from './TextGeomFields.svelte'
  import FontPicker from './FontPicker.svelte'
  import FieldMenu from '../captions/FieldMenu.svelte'
  import { fieldsOf } from '../captions/fields'
  import CaptionSheet from '../captions/CaptionSheet.svelte'
  import { MAX_PAD, PAD_STEP, padOf } from '../captions/sheetcaps'

  // Карточка выделенного блока или подписи (спека 008, из TextToolbar): шрифт, кегль, цвет,
  // выравнивание, Ж/К; у блока — угол, «По тексту», «Привязка»; у подписи — «Под фото / На фото», отступ
  // и «Один стиль на лист» (030). Поля {Фамилия}… вставляет меню «Вставить поле» (в блок и в подпись).
  // Правки идут в Go по одной (шаг undo на действие); цвет — на change, не на каждое движение в палитре.
  // head=false — заголовок рисует сама карточка (docked, cardTitle): сворачиваемая шапка.
  let { head = true }: { head?: boolean } = $props()
  const st = $derived(edit.style)
  const cap = $derived(edit.caption)
  const blk = $derived(edit.text)
  const sup = $derived(st ? styleSupport(st.font, store.fonts) : { bold: false, italic: false })

  const ALIGNS = [
    { v: 'left', title: 'По левому краю', d: 'M4 6h16M4 10h10M4 14h16M4 18h10' },
    { v: 'center', title: 'По центру', d: 'M4 6h16M7 10h10M4 14h16M7 18h10' },
    { v: 'right', title: 'По правому краю', d: 'M4 6h16M10 10h10M4 14h16M10 18h10' },
  ] as const

  // Кнопки не забирают фокус у редактора: курсор остаётся в тексте.
  const keepFocus = (e: MouseEvent) => e.preventDefault()
</script>

{#if st}
  {#if head}<div class="head">{cap ? 'Подпись' : 'Текст'}</div>{/if}
  <FontPicker value={st.font} fonts={store.fonts} onpick={(id) => edit.setStyle({ font: id })} />

  <div class="row">
    <div class="size" title="Кегль, pt">
      <button
        class="sq"
        aria-label="Меньше"
        disabled={st.size <= MIN_SIZE}
        onmousedown={keepFocus}
        onclick={() => edit.setStyle({ size: st.size - 1 })}>−</button
      >
      <input
        type="number"
        min={MIN_SIZE}
        max={MAX_SIZE}
        step="1"
        aria-label="Кегль, pt"
        value={st.size}
        onchange={(e) => {
          const v = clampSize(Number(e.currentTarget.value), st.size)
          e.currentTarget.value = String(v)
          edit.setStyle({ size: v })
        }}
      />
      <button
        class="sq"
        aria-label="Больше"
        disabled={st.size >= MAX_SIZE}
        onmousedown={keepFocus}
        onclick={() => edit.setStyle({ size: st.size + 1 })}>+</button
      >
    </div>
    <ColorPicker value={st.color} label="Цвет текста" onpick={(c) => edit.setStyle({ color: c })} />
  </div>

  <div class="row">
    <div class="seg">
      {#each ALIGNS as a (a.v)}
        <button
          class="sq"
          class:on={st.align === a.v}
          title={a.title}
          aria-label={a.title}
          aria-pressed={st.align === a.v}
          onmousedown={keepFocus}
          onclick={() => edit.setStyle({ align: a.v })}
        >
          <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"
            ><path d={a.d} /></svg
          >
        </button>
      {/each}
    </div>
    <div class="seg">
      <button
        class="sq b"
        class:on={st.bold}
        disabled={!sup.bold}
        title={sup.bold ? 'Жирный' : 'У этого шрифта нет жирного начертания'}
        aria-label="Жирный"
        aria-pressed={!!st.bold}
        onmousedown={keepFocus}
        onclick={() => edit.setStyle({ bold: !st.bold })}>Ж</button
      >
      <button
        class="sq i"
        class:on={st.italic}
        disabled={!sup.italic}
        title={sup.italic ? 'Курсив' : 'У этого шрифта нет курсива'}
        aria-label="Курсив"
        aria-pressed={!!st.italic}
        onmousedown={keepFocus}
        onclick={() => edit.setStyle({ italic: !st.italic })}>К</button
      >
    </div>
  </div>

  {#if blk}
    <div class="row"><TextGeomFields t={blk} /></div>
    {#if prefs.can('manual')}<SnapToggle />{/if}
    <!-- Слой текста — в общем стеке с фото (может лечь под фото); только «Ручная» -->
    {#if placesel.canLayer && prefs.can('layers')}
      <LayerRow onpick={(op) => placesel.layer(op)} />
    {/if}
  {/if}
  <FieldMenu onpick={(f) => edit.insertField(f)} fields={fieldsOf(edit.spread)} />
  {#if cap}
    <button class="txt" title="Подпись стоит под фото или на самом фото" onmousedown={keepFocus} onclick={() => edit.setOver(!cap.over)}>
      {cap.over ? 'На фото' : 'Под фото'}
    </button>
    <div class="row">
      <label for="cap-pad" title="Расстояние от фото до подписи; на фото — от нижнего края">Отступ, мм</label>
      <input
        id="cap-pad"
        class="pad"
        type="number"
        min="0"
        max={MAX_PAD}
        step={PAD_STEP}
        value={padOf(cap)}
        onchange={async (e) => {
          const el = e.currentTarget
          // Недонабранное или зажатое Go значение возвращаем в поле после ответа.
          await edit.setPad(el.valueAsNumber)
          el.value = String(padOf(edit.caption))
        }}
      />
    </div>
    {#if edit.spread}<CaptionSheet sp={edit.spread} />{/if}
  {/if}

  <button class="del" onmousedown={keepFocus} onclick={() => edit.removeSelected()} title="Удалить (Delete / Backspace)"
    ><Icon name="trash" size={14} />Удалить</button
  >
{/if}

<style>
  .head {
    font-size: 13px;
    font-weight: 600;
    margin-bottom: 8px;
  }
  .row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 6px;
    margin-top: 8px;
  }
  .size {
    display: flex;
    align-items: center;
    gap: 2px;
  }
  .size input {
    width: 46px;
    height: 24px;
    padding: 0 2px;
    font-size: 12px;
    text-align: center;
  }
  .row label {
    font-size: 12px;
    color: var(--muted);
  }
  .pad {
    width: 64px;
    height: 24px;
    padding: 0 4px;
    box-sizing: border-box;
    text-align: right;
    font-size: 12px;
  }
  .seg {
    display: flex;
    gap: 1px;
  }
  .sq {
    flex: none;
    width: 26px;
    height: 24px;
    padding: 0;
    display: grid;
    place-items: center;
    background: transparent;
    color: #cfcfd4;
    border-radius: 4px;
    font-size: 14px;
  }
  .sq.on {
    background: var(--accent);
    color: #1e1e1e;
  }
  .sq.on:hover:not(:disabled) {
    background: #ffe53a;
  }
  .b {
    font-weight: 700;
  }
  .i {
    font-style: italic;
  }
  .txt {
    margin-top: 8px;
    width: 100%;
    height: 24px;
    padding: 0 8px;
    font-size: 12px;
    background: #3a3a3e;
  }
  .del {
    margin-top: 10px;
    width: 100%;
    height: 26px;
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 6px;
    font-size: 12px;
    background: #38383c;
    color: #cfcfd4;
  }
  .del:hover:not(:disabled) {
    color: #ff8a8a;
  }
</style>
