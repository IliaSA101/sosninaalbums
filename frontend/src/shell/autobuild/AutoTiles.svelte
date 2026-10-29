<script lang="ts">
  import type { Unit } from '../../base/geom/sheet'
  import type { AutoDialog } from './autobuild.svelte'
  import { anchorField, otherField, perText, planHint, rangeText, unitWord } from './autobuild'

  // Две связанные карточки окна «Автосборка» (027 U2): число разворотов N и среднее «фото на разворот», между ними ⇄.
  // Якорь — карточка, из которой считается вторая. typing — какое поле сейчас печатают (общее с клавишами окна, поэтому bindable).
  let {
    ui,
    unit,
    empty,
    low,
    top,
    typing = $bindable(null),
  }: {
    ui: AutoDialog
    unit: Unit
    empty: boolean
    low: number // пределы N для поля числа
    top: number
    typing: 'n' | 'per' | null
  } = $props()

  // Пока в поле печатают, в нём остаётся набранное (draft); иначе — фактическое значение, даже при фокусе:
  // ответ Go мог поправить N после открытия, а поле с автофокусом не должно застрять на старом числе.
  // Якорь меняет только действие пользователя (щелчок по карточке, ⇄, ввод); простой фокус — нет:
  // иначе автофокус при открытии и возврат фокуса после окна поверх затирали бы запомненное «среднее».
  let draft = $state('')
  const enter = (e: FocusEvent) => (e.currentTarget as HTMLInputElement).select()
  // ⇄ переводит ввод во второе поле: оно становится активным, значение пересчитается от первого.
  const swap = () => {
    const to = otherField(ui.anchor) // до activate: после него якорь уже другой
    ui.activate(ui.anchor === 'spreads' ? 'per' : 'spreads')
    document.getElementById(to)?.focus()
  }
  // Щелчок по карточке (в том числе вне поля) делает её поле якорем; ⇄ обрабатывает swap.
  function pick(e: Event) {
    const f = (e.target as Element).closest('label.tile')?.getAttribute('for')
    if (f) ui.activate(f === 'auto-n' ? 'spreads' : 'per')
  }
  // Поле якоря получает первый фокус (data-autofocus); у второго атрибута нет вовсе — false Svelte пишет строкой.
  const first = (id: 'auto-n' | 'auto-per') => (anchorField(ui.anchor) === id ? '' : undefined)
  const shownPer = () => perText(ui.photos, ui.n)
</script>

<div class="tiles">
  <div class="pair" role="presentation" onpointerdown={pick} onclick={pick}>
    <label class="tile" for="auto-n" class:anchor={ui.anchor === 'spreads'}>
      <span class="row">
        <input
          id="auto-n"
          class="big"
          type="number"
          min={low}
          max={top}
          step="1"
          value={typing === 'n' ? draft : ui.n}
          disabled={empty}
          data-autofocus={first('auto-n')}
          aria-label={unitWord(ui.n, unit)}
          onfocus={enter}
          onblur={() => (typing = null)}
          oninput={(e) => {
            typing = 'n'
            draft = e.currentTarget.value
            if (draft !== '') ui.setSpreads(e.currentTarget.valueAsNumber)
          }}
        />
        <span class="cap">{unitWord(ui.n, unit)}</span>
      </span>
      <span class="sub">{unit.one === 'лист' ? '' : `${2 * ui.n} стр.`}</span>
      <span class="rem">{ui.anchor === 'spreads' ? 'Запомню для следующей сборки' : ''}</span>
    </label>

    <button class="link" type="button" disabled={empty} title="Задать другим полем" aria-label="Переключить поле ввода" onclick={swap}
      >⇄</button
    >

    <label class="tile" for="auto-per" class:anchor={ui.anchor === 'per'} title={ui.plan && !ui.pending ? planHint(ui.plan, unit) : ''}>
      <span class="row">
        <input
          id="auto-per"
          class="big"
          type="text"
          inputmode="decimal"
          autocomplete="off"
          data-autofocus={first('auto-per')}
          value={empty ? '' : typing === 'per' ? draft : shownPer()}
          disabled={empty}
          aria-label="Фото на {unit.one}"
          onfocus={enter}
          onblur={() => (typing = null)}
          oninput={(e) => {
            typing = 'per'
            draft = e.currentTarget.value
            ui.setPer(Number(draft.replace(',', '.')))
          }}
        />
        <span class="cap">фото на {unit.one}</span>
      </span>
      <span class="sub" role="status">
        {#if ui.error}
          <span class="err">{ui.error}</span>
        {:else if empty}
          Нет фото для сборки
        {:else if ui.pending || !ui.plan}
          Считаю…
        {:else}
          {rangeText(ui.plan)}
        {/if}
      </span>
      <span class="rem">{ui.anchor === 'per' ? 'Запомню для следующей сборки' : ''}</span>
    </label>
  </div>
</div>

<style>
  /* Поля встают в ряд, пока хватает места; иначе друг под другом (без прокрутки и обрезки).
     Порог — окно ≤ 600 (спека 065): контейнер = окно − поля Modal (2×16) − рамка (2) − 32 отступа от края = 600 − 66 = 534. */
  .tiles {
    container-type: inline-size;
  }
  .pair {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto minmax(0, 1fr);
    align-items: center;
    gap: 8px;
  }
  @container (max-width: 534px) {
    .pair {
      grid-template-columns: minmax(0, 1fr);
    }
    .link {
      justify-self: center;
      transform: rotate(90deg);
    }
  }
  .tile {
    display: block;
    cursor: pointer; /* вся карточка — цель щелчка: фокус уходит в её поле */
    min-width: 0;
    padding: 8px 10px;
    border: 1px solid var(--line);
    border-radius: 8px;
    background: var(--panel2);
  }
  .tile:hover {
    border-color: var(--muted);
  }
  .tile.anchor,
  .tile:focus-within {
    border-color: var(--accent);
  }
  .row {
    display: flex;
    flex-wrap: wrap;
    align-items: baseline;
    gap: 4px 8px;
  }
  .cap {
    font-size: 13px;
  }
  .big {
    width: 76px;
    font: inherit;
    font-size: 20px;
    font-weight: 600;
    text-align: center;
    height: 36px;
    box-sizing: border-box;
    background: #3a3a3e;
    color: var(--text);
    border: 1px solid var(--line);
    border-radius: 6px;
    padding: 0 4px;
  }
  .big:focus {
    outline: 1px solid var(--accent);
  }
  .sub {
    display: block;
    margin-top: 4px;
    min-height: 18px;
    color: var(--muted);
    font-size: 12px;
  }
  .rem {
    display: block;
    min-height: 14px;
    color: var(--muted);
    font-size: 11px;
  }
  .link {
    padding: 2px 6px;
    background: transparent;
    border: 0;
    color: var(--muted);
    font-size: 18px;
    cursor: pointer;
  }
  .link:hover:not(:disabled) {
    color: var(--text);
  }
  .err {
    color: #e5645a;
  }
</style>
