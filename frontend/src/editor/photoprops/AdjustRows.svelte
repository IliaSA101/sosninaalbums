<script lang="ts">
  import { Api, store } from '../../base/core/store.svelte'
  import { adjlive } from '../photo/adjlive.svelte'
  import { toneCss } from '../../base/geom/tone'
  import Icon from '../../base/ui/Icon.svelte'
  import { canReset, clampRow, needsGray, pendingGate, signed, toneArgs, toneOf, withRow, type ToneRow } from './adjustrows'

  // «Цвет | Ч/б», «Яркость» и «Контраст» выделенного фото (спека 068): образец — «Масштаб» (число, ⊗, ползунок) и
  // «Целиком / С обрезкой». Тянут — живой показ через adjlive (Go не зовём), отпустили — один SetTone (один шаг отмены).
  let { id }: { id: string } = $props()

  const sp = $derived(store.spread)
  const fv = $derived(sp ? sp.frames?.[sp.photos.indexOf(id)] : undefined)
  const base = $derived(toneOf(fv))
  const live = $derived(adjlive.of(id))
  const cur = $derived({ ...base, ...live })
  const ROWS: { row: ToneRow; label: string }[] = [
    { row: 'bright', label: 'Яркость' },
    { row: 'contrast', label: 'Контраст' },
  ]

  // Пока SetTone/SetGray не вернулся — новые щелчки и отпускания игнорируем (двойной «Ч/б» переключил бы дважды).
  const gate = pendingGate()
  const val = (e: Event) => clampRow(+(e.currentTarget as HTMLInputElement).value)

  function liveSet(row: ToneRow, e: Event) {
    adjlive.set(id, withRow(base, row, val(e)))
  }
  // Отпустили: если значение изменилось — один SetTone; просмотр гасим после ответа Go (иначе кадр моргнёт).
  async function commit(row: ToneRow, e: Event) {
    const args = toneArgs(base, row, val(e))
    const ran = await gate.run(async () => {
      if (args) await store.run(Api.SetTone(id, args[0], args[1]))
      adjlive.clear()
    })
    if (!ran) adjlive.clear() // правку при чужом вызове отбросили — просмотр не залипает
  }
  // Chromium не шлёт change, если при отпускании значение вернулось к исходному: просмотр не должен залипнуть.
  // Если значение отличается — change на подходе, просмотр гасит он.
  function blur() {
    if (toneCss(adjlive.of(id)) === toneCss(base)) adjlive.clear()
  }
  async function reset(row: ToneRow) {
    const args = toneArgs(base, row, 0)
    await gate.run(async () => {
      if (args) await store.run(Api.SetTone(id, args[0], args[1]))
      adjlive.clear()
    })
  }
  async function setGray(want: boolean) {
    if (!needsGray(base, want)) return
    await gate.run(() => store.run(Api.SetGray([id])))
  }
  // Смена фото или уход панели: чужой просмотр не оставляем.
  $effect(() => {
    void id
    return () => adjlive.clear()
  })
</script>

<div class="adj" data-guide="adjust">
  <div class="seg" role="group" aria-label="Цвет фото">
    <button class:on={!cur.gray} aria-pressed={!cur.gray} title="Цветное фото" onclick={() => setGray(false)}>Цвет</button>
    <button class:on={cur.gray} aria-pressed={cur.gray} title="Чёрно-белое фото" onclick={() => setGray(true)}>Ч/б</button>
  </div>
  {#each ROWS as { row, label } (row)}
    <div class="row">
      <span class="lbl">{label}</span>
      <span class="num">{signed(cur[row])}</span>
      <button
        class="reset"
        disabled={!canReset(cur[row])}
        onclick={() => reset(row)}
        aria-label="Сбросить: {label.toLowerCase()}"
        title="Сбросить: {label.toLowerCase()}"><Icon name="reset" size={16} /></button
      >
    </div>
    <input
      type="range"
      min="-50"
      max="50"
      step="1"
      value={cur[row]}
      aria-label={label}
      aria-valuetext={signed(cur[row])}
      oninput={(e) => liveSet(row, e)}
      onchange={(e) => commit(row, e)}
      onblur={blur}
      onpointerup={(e) => e.currentTarget.blur()}
    />
  {/each}
</div>

<style>
  .adj {
    margin-top: 10px;
  }
  .seg {
    display: flex;
    border-radius: 5px;
    overflow: hidden;
    gap: 1px;
  }
  .seg button {
    flex: 1;
    height: 26px;
    padding: 0 6px;
    font-size: 12px;
    border-radius: 0;
    background: #38383c;
    color: #cfcfd4;
  }
  .seg button.on {
    background: var(--accent);
    color: #1e1e1e;
  }
  .seg button.on:hover {
    background: #ffe53a;
  }
  .row {
    display: flex;
    align-items: center;
    gap: 6px;
    margin-top: 10px;
    font-size: 12px;
  }
  .lbl {
    color: var(--muted);
    flex: 1;
  }
  .num {
    min-width: 28px;
    text-align: right;
    font-variant-numeric: tabular-nums;
  }
  .reset {
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
