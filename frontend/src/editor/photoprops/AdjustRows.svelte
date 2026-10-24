<script lang="ts">
  import { Api, store } from '../../base/core/store.svelte'
  import { adjlive } from '../photo/adjlive.svelte'
  import { MAX_TONE, toneCss } from '../../base/geom/tone'
  import TiltRow from './TiltRow.svelte'
  import { clampRow, needsGray, pendingGate, toneArgs, toneOf, withRow, type ToneRow } from './adjustrows'

  // «Цвет | Ч/б», «Яркость» и «Контраст» выделенного фото (спека 068): образец — «Масштаб» и «Целиком / С обрезкой».
  // Ползунки — общая строка TiltRow с числом и ⊗ (124 U4). Тянут — живой показ через adjlive (Go не зовём),
  // отпустили — один SetTone (один шаг отмены).
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
  function liveSet(row: ToneRow, v: number) {
    adjlive.set(id, withRow(base, row, clampRow(v)))
  }
  // Отпустили: если значение изменилось — один SetTone; просмотр гасит onend после ответа Go (иначе кадр моргнёт),
  // в том числе когда правку при чужом вызове отбросили — просмотр не залипает. ⊗ — тот же путь со значением 0.
  async function commit(row: ToneRow, v: number) {
    const args = toneArgs(base, row, v)
    await gate.run(async () => {
      if (args) await store.run(Api.SetTone(id, args[0], args[1]))
    })
  }
  // Chromium не шлёт change, если при отпускании значение вернулось к исходному: просмотр не должен залипнуть.
  // Если значение отличается — change на подходе, просмотр гасит он.
  function blur() {
    if (toneCss(adjlive.of(id)) === toneCss(base)) adjlive.clear()
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
    <TiltRow
      tilt={base[row]}
      {label}
      numLabel="{label}, значение"
      rangeLabel={label}
      rangeTitle="{label}; двойной щелчок — сбросить"
      min={-MAX_TONE}
      max={MAX_TONE}
      step={1}
      guide=""
      id="tone-{row}"
      reset
      onlive={(v) => liveSet(row, v)}
      oncommit={(v) => commit(row, v)}
      onend={() => adjlive.clear()}
      onleave={blur}
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
</style>
