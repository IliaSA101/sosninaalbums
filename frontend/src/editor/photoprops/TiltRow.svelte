<script lang="ts">
  import Icon from '../../base/ui/Icon.svelte'
  import { fieldKey, leftRow } from './tiltfocus'
  import { TILT_MAX } from '../tilt/tiltdial'

  // Строка «Наклон, °» (010 U5 → 045): поле, ползунок, двойной щелчок — 0, кнопка «Выпрямить по линии». Общая
  // для карточки фото (InspectorFrame) и панели фона (BackdropFields). Ползунок и поле ведут предпросмотр
  // (onlive), правка уходит на отпускании и по Enter / уходу из поля (oncommit); onend — всегда после, сбросить
  // предпросмотр. Четверти и наклон разложил Go, R/90 здесь не считается.
  // С параметрами подписи, пределов и шага (044) это же и «Скругление, %»: tilt — текущее значение строки,
  // onstraighten нет — нет и кнопки. По умолчанию — прежний «Наклон».
  let {
    tilt,
    onlive,
    oncommit,
    onend,
    onleave,
    onstraighten,
    label = 'Наклон, °',
    numLabel = 'Наклон, градусы',
    rangeLabel = 'Наклон',
    rangeTitle = 'Наклон фото в рамке; двойной щелчок — выровнять',
    min = -TILT_MAX,
    max = TILT_MAX,
    step = 0.1,
    guide = 'tilt',
    id = 'tilt',
    dflt = 0,
    mixed = false,
  }: {
    tilt: number
    onlive: (t: number) => void
    oncommit: (t: number) => Promise<unknown> | unknown
    onend: () => void
    onleave?: () => void // фокус ушёл из строки: сессия жеста (057) кончилась
    onstraighten?: () => void
    label?: string
    numLabel?: string
    rangeLabel?: string
    rangeTitle?: string
    min?: number
    max?: number
    step?: number
    guide?: 'tilt' | 'round' | '' // якорь справочника F1 (литералами в разметке: страж ищет их текстом); '' — без якоря
    id?: string
    dflt?: number | null // двойной щелчок по ползунку (079: «по умолчанию»); null — двойной щелчок молчит
    mixed?: boolean // у выделенных значения разные (079, группа): в поле «—», первая правка ставит всем одно
  } = $props()

  let wrap = $state<HTMLElement>() // обёртка строки: уход фокуса ловим со всей строки (ползунок ↔ поле — не уход)
  function focusout(e: FocusEvent) {
    if (wrap && leftRow(wrap, e.relatedTarget)) onleave?.()
  }

  let draft = $state<number | null>(null) // что сейчас показано в поле, пока тянут
  const shown = $derived(draft ?? tilt)
  // Пустое поле или «-» (valueAsNumber — NaN): ввод не закончен, поле и draft не трогаем, иначе минус не набрать.
  const tiltOf = (e: Event) => (e.currentTarget as HTMLInputElement).valueAsNumber
  const k = $derived(Math.round(1 / step)) // 0,1 → ×10, 1 → ×1
  const clampTilt = (v: number) => Math.min(max, Math.max(min, Math.round(v * k) / k))

  function live(e: Event) {
    const v = tiltOf(e)
    if (!Number.isFinite(v)) return
    draft = clampTilt(v)
    onlive(draft)
  }
  async function commit(e: Event) {
    const input = e.currentTarget as HTMLInputElement
    const v = tiltOf(e)
    if (Number.isFinite(v) && (mixed || clampTilt(v) !== tilt)) await oncommit(clampTilt(v))
    // Недонабранное («-», пусто) откатываем к значению из Go, иначе поле осталось бы пустым.
    else if (!Number.isFinite(v)) input.value = mixed ? '' : String(tilt)
    draft = null // после ответа Go: иначе ползунок на миг прыгнет назад
    onend()
  }
  // Enter применяет (change придёт при blur) и снимает фокус, как в BackdropFields; Esc — вернуть значение из Go,
  // выделение не снимать (как в GroupFields/PlaceFields). Фокус уходит из поля, но не из строки — сессия цела.
  function key(e: KeyboardEvent) {
    const act = fieldKey(e.key)
    if (!act) return
    const input = e.currentTarget as HTMLInputElement
    if (act === 'cancel') {
      e.stopPropagation()
      input.value = mixed ? '' : String(tilt)
      draft = null
      onend()
    }
    input.blur()
  }
</script>

{#snippet head()}
  <label class="lbl" for="{id}-num">{label}</label>
  {#if onstraighten}
    <button
      class="level"
      data-guide="straighten"
      onclick={onstraighten}
      aria-label="Выпрямить по линии"
      title="Выпрямить по линии: проведите линию вдоль горизонта или вертикали"><Icon name="level" size={16} /></button
    >
  {/if}
  <input
    id="{id}-num"
    class="num"
    type="number"
    {min}
    {max}
    {step}
    value={mixed && draft === null ? '' : shown}
    placeholder={mixed ? '—' : undefined}
    aria-label={numLabel}
    oninput={live}
    onchange={commit}
    onkeydown={key}
  />
{/snippet}
<!-- display: contents — обёртка не меняет раскладку родителя -->
<div class="wrap" bind:this={wrap} onfocusout={focusout}>
  {#if guide === 'round'}
    <div class="row tilt" data-guide="round">{@render head()}</div>
  {:else if guide === 'tilt'}
    <div class="row tilt" data-guide="tilt">{@render head()}</div>
  {:else}
    <div class="row tilt">{@render head()}</div>
  {/if}
  <input
    type="range"
    {min}
    {max}
    {step}
    value={shown}
    aria-label={rangeLabel}
    title={rangeTitle}
    oninput={live}
    onchange={commit}
    ondblclick={(e) => {
      if (dflt === null) return
      e.currentTarget.value = String(dflt)
      live(e)
      commit(e)
    }}
    onpointerup={(e) => e.currentTarget.blur()}
  />
</div>

<style>
  .wrap {
    display: contents;
  }
  .row {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 12px;
  }
  .lbl {
    color: var(--muted);
    flex: 1;
    white-space: nowrap; /* кнопка-уровень не должна переносить подпись в узкой панели */
  }
  .tilt {
    margin-top: 8px;
  }
  .num {
    width: 64px;
    min-width: 0;
    height: 24px;
    padding: 0 4px;
    box-sizing: border-box;
    text-align: right;
    font-variant-numeric: tabular-nums;
  }
  .level {
    flex: none;
    width: 24px;
    height: 24px;
    padding: 0;
    display: grid;
    place-items: center;
    background: transparent;
    color: #cfcfd4;
    border-radius: 4px;
  }
  input[type='range'] {
    width: 100%;
    margin: 6px 0 0;
  }
</style>
