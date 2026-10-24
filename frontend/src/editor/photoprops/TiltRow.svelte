<script lang="ts">
  import Icon from '../../base/ui/Icon.svelte'
  import { useCard } from '../../base/prefs/cardgroups.svelte'
  import { fieldKey, leftRow } from './tiltfocus'
  import { TILT_MAX } from '../tilt/tiltdial'

  // Строка «Наклон, °» (010 U5 → 045): поле, ползунок, двойной щелчок — 0, кнопка «Выпрямить по линии». Общая
  // для карточки фото (InspectorFrame) и панели фона (BackdropFields). Ползунок и поле ведут предпросмотр
  // (onlive), правка уходит на отпускании и по Enter / уходу из поля (oncommit); onend — всегда после, сбросить
  // предпросмотр. Четверти и наклон разложил Go, R/90 здесь не считается.
  // С параметрами подписи, пределов и шага (044) это же и «Скругление, %»: tilt — текущее значение строки,
  // onstraighten нет — нет и кнопки. По умолчанию — прежний «Наклон».
  // Один вид ползунка карточки (124 U4): Масштаб, Яркость, Контраст, Прозрачность — тоже эта строка. reset добавляет
  // ⊗ справа от числа (по умолчанию oncommit(dflt), onreset — свой сброс, как ResetFrame у масштаба); onstart — фокус
  // вошёл в строку. В панели деталей строка одна: подпись · дорожка · число · ⊗; в узкой карточке дорожка второй строкой.
  let {
    tilt,
    onlive,
    oncommit,
    onend,
    onleave,
    onstart,
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
    reset = false,
    onreset,
    resetOff,
    resetLabel,
    resetTitle,
  }: {
    tilt: number
    onlive: (t: number) => void
    oncommit: (t: number) => Promise<unknown> | unknown
    onend: () => void
    onleave?: () => void // фокус ушёл из строки: сессия жеста (057) кончилась
    onstart?: () => void // фокус вошёл в строку: сессия жеста началась (пара к onleave)
    onstraighten?: () => void
    label?: string
    numLabel?: string
    rangeLabel?: string
    rangeTitle?: string
    min?: number
    max?: number
    step?: number
    guide?: 'tilt' | 'round' | 'photo-fade' | '' // якорь справочника F1 (литералами в разметке: страж ищет их текстом); '' — без якоря
    id?: string
    dflt?: number | null // двойной щелчок по ползунку (079: «по умолчанию»); null — двойной щелчок молчит
    mixed?: boolean // у выделенных значения разные (079, группа): в поле «—», первая правка ставит всем одно
    reset?: boolean // ⊗ справа от числа
    onreset?: () => unknown // свой сброс (ResetFrame у масштаба); без него — oncommit(dflt)
    resetOff?: boolean // ⊗ серая; без него — значение уже по умолчанию (или dflt нет)
    resetLabel?: string // aria-label ⊗; без него — «Сбросить: <подпись ползунка>»
    resetTitle?: string
  } = $props()

  const card = useCard()
  const line = $derived(!card.narrow) // панель деталей: одна строка; плавающая карточка: дорожка второй строкой
  const rstText = $derived(resetLabel ?? `Сбросить: ${rangeLabel.toLowerCase()}`)
  const rstOff = $derived(resetOff ?? (dflt === null || (!mixed && tilt === dflt)))

  let wrap = $state<HTMLElement>() // обёртка строки: уход фокуса ловим со всей строки (ползунок ↔ поле — не уход)
  function focusout(e: FocusEvent) {
    if (wrap && leftRow(wrap, e.relatedTarget)) onleave?.()
  }
  function focusin(e: FocusEvent) {
    if (wrap && leftRow(wrap, e.relatedTarget)) onstart?.()
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
  // ⊗ — отдельное действие: серия протяжек до него кончается (onleave), сброс — свой шаг отмены.
  async function doReset() {
    onleave?.()
    if (onreset) await onreset()
    else if (dflt !== null) await oncommit(dflt)
    draft = null
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

{#snippet cell()}
  <span class="cell">
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
  </span>
{/snippet}
{#snippet num()}
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
{#snippet rst()}
  {#if reset}
    <button class="reset" disabled={rstOff} onclick={doReset} aria-label={rstText} title={resetTitle ?? rstText}
      ><Icon name="reset" size={16} /></button
    >
  {/if}
{/snippet}
{#snippet track()}
  <input
    type="range"
    class="track"
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
{/snippet}
{#snippet body()}
  {#if line}
    {@render cell()}{@render track()}{@render num()}{@render rst()}
  {:else}
    {@render cell()}{@render num()}{@render rst()}{@render track()}
  {/if}
{/snippet}
<div class="wrap" bind:this={wrap} onfocusout={focusout} onfocusin={focusin}>
  {#if guide === 'round'}
    <div class="box" class:line data-guide="round">{@render body()}</div>
  {:else if guide === 'tilt'}
    <div class="box" class:line data-guide="tilt">{@render body()}</div>
  {:else if guide === 'photo-fade'}
    <div class="box" class:line data-guide="photo-fade">{@render body()}</div>
  {:else}
    <div class="box" class:line>{@render body()}</div>
  {/if}
</div>

<style>
  .wrap {
    margin-top: 8px;
  }
  .box {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 6px;
    font-size: 12px;
  }
  .box.line {
    flex-wrap: nowrap;
  }
  /* Плавающая карточка (200 px): подпись целиком, не режется; не влезло число с ⊗ — уходят строкой ниже, дорожка за ними. */
  .cell {
    flex: 1 0 auto;
    display: flex;
    align-items: center;
    gap: 6px;
  }
  .line .cell {
    flex: 0 0 104px; /* подпись и «Выпрямить» в общей ячейке: дорожки всех строк начинаются с одного места */
    min-width: 0;
  }
  .lbl {
    color: var(--muted);
    flex: none;
    white-space: nowrap; /* кнопка-уровень не должна переносить подпись */
  }
  .line .lbl {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis; /* в панели ширина ячейки задана, подпись при нужде обрезается */
  }
  .num {
    width: 56px;
    min-width: 0;
    height: 24px;
    padding: 0 4px;
    box-sizing: border-box;
    text-align: right;
    font-variant-numeric: tabular-nums;
  }
  .line .num {
    width: 64px;
  }
  .level,
  .reset {
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
  .track {
    flex: 1 0 100%;
    width: 100%;
    margin: 0;
  }
  .line .track {
    flex: 1 1 0;
    min-width: 40px;
    width: auto;
  }
</style>
