<script lang="ts" module>
  // Пресеты страницы статичны: Go отдаёт их списком, читаем один раз за запуск (как coverAct.loadPresets).
  let loaded: PagePreset[] | null = null
</script>

<script lang="ts">
  import { onMount, untrack } from 'svelte'
  import { Api, store } from '../../base/core/store.svelte'
  import type { PagePreset, Settings } from '../../base/core/types'
  import ColorPicker from '../../base/ui/ColorPicker.svelte'
  import { SHEET_PRESETS, isSheet, newSheetSize } from '../../base/geom/sheet'
  import { presetActive, presetFields, presetRows, exportPx, type Preset } from './presets'

  const NUMS = ['pageW', 'pageH', 'margin', 'gap', 'dpi', 'safe'] as const

  // Поле number при пустом вводе даёт null — приводим к числу при сравнении и сохранении.
  const norm = (x: Settings): Settings => ({
    pageW: Number(x.pageW),
    pageH: Number(x.pageH),
    margin: Number(x.margin),
    gap: Number(x.gap),
    dpi: Number(x.dpi),
    safe: Number(x.safe),
    layflat: !!x.layflat,
    bg: x.bg ?? '', // "" — белый
    sheet: x.sheet, // Go режим не меняет: лист остаётся листом
  })
  const same = (a: Settings, b: Settings) => {
    const x = norm(a)
    const y = norm(b)
    return x.layflat === y.layflat && x.bg === y.bg && NUMS.every((k) => x[k] === y[k])
  }

  // base — настройки, с которыми форма сверялась последний раз.
  let base = $state<Settings>({ ...store.view.settings })
  let s = $state<Settings>({ ...store.view.settings })

  const edited = $derived(!same(s, base))
  const differs = $derived(!same(s, store.view.settings))
  const valid = $derived.by(() => {
    const n = norm(s)
    return n.pageW > 0 && n.pageH > 0 && n.dpi > 0 && n.margin >= 0 && n.gap >= 0 && n.safe >= 0 && NUMS.every((k) => Number.isFinite(n[k]))
  })

  // Настройки сменились не отсюда (Undo, второе окно): без своих правок форма подтягивает их.
  $effect(() => {
    const cur = store.view.settings
    untrack(() => {
      if (same(cur, base)) return
      if (!edited) s = { ...cur }
      base = { ...cur }
    })
  })

  // Один вызов — один шаг отмены; живое применение забило бы историю пересчётами.
  async function apply() {
    await store.setSettings(norm(s))
    s = { ...store.view.settings }
    base = { ...store.view.settings }
  }

  // Пресет только заполняет форму; применяет «Применить» — один шаг undo.
  function pick(p: Preset) {
    s = { ...s, ...presetFields(p) }
  }

  // Лист виньетки (031): размер листа вместо страницы, пресеты 20×30 и 30×40, layflat и «Свой» не нужны.
  const sheet = $derived(isSheet(store.view.settings))
  let presets = $state<PagePreset[]>(loaded ?? [])
  const rows = $derived(sheet ? [{ group: '', items: [...SHEET_PRESETS] }] : presetRows(presets))

  // Не удалось получить — формы это не мешает: остаётся ввод руками.
  onMount(() => {
    if (loaded) return
    Api.PagePresets()
      .then((l) => (presets = loaded = (l ?? []) as PagePreset[]))
      .catch(() => {})
  })

  const activeHint = $derived(presets.find((p) => p.hint && presetActive(p, s))?.hint)

  function reset() {
    s = { ...store.view.settings }
    base = { ...store.view.settings }
  }
</script>

{#each rows as row (row.group)}
  <div class="presets" class:fs={row.group} {...row.group ? { 'data-guide': 'fsfera-presets' } : {}}>
    {#if row.group}<span class="cap">{row.group}:</span>{/if}
    {#each row.items as p (p.label)}
      <button class:active={presetActive(p, s)} onclick={() => pick(p)}>{p.label}</button>
    {/each}
  </div>
{/each}

<div class="form" data-guide="settings-form">
  {#if sheet}
    <label
      >Лист, мм: Ш × В
      <span class="pair"
        ><input type="number" min="50" max="1000" aria-label="Ширина листа, мм" bind:value={s.pageW} /> ×
        <input type="number" min="50" max="1000" aria-label="Высота листа, мм" bind:value={s.pageH} /></span
      ></label
    >
  {:else}
    <label>Ширина страницы, мм <input type="number" min="100" max="600" bind:value={s.pageW} /></label>
    <label>Высота страницы, мм <input type="number" min="100" max="600" bind:value={s.pageH} /></label>
  {/if}
  <label>Поля, мм <input type="number" min="0" max="60" step="0.5" bind:value={s.margin} /></label>
  <label title="Значимое — не ближе к краю; пунктир — Вид → Безопасная зона"
    >Безопасная зона, мм <input type="number" min="0" max="30" step="0.5" bind:value={s.safe} /></label
  >
  <label>Промежуток между фото, мм <input type="number" min="0" max="30" step="0.5" bind:value={s.gap} /></label>
  <label>Разрешение экспорта, dpi <input type="number" min="150" max="600" step="50" bind:value={s.dpi} /></label>
  <label>
    {sheet ? 'Цвет листов' : 'Цвет страниц альбома'}
    <!-- цвет попадает в форму при выборе; применяет «Применить» -->
    <ColorPicker value={s.bg || '#ffffff'} label={sheet ? 'Цвет листов' : 'Цвет страниц альбома'} onpick={(c) => (s.bg = c)} />
  </label>
  {#if !sheet}
    <label class="check">
      <input type="checkbox" bind:checked={s.layflat} />
      Фото может проходить через корешок (layflat-альбом)
    </label>
  {/if}
</div>
{#if valid}
  {@const sh = newSheetSize({ pageW: +s.pageW, pageH: +s.pageH, sheet: s.sheet })}
  <p class="note">
    {sh.mid > 0 ? 'Разворот = две страницы' : 'Лист'}: {sh.w / 10} × {sh.h / 10} см, при экспорте {exportPx(sh.w, +s.dpi)} ×
    {exportPx(sh.h, +s.dpi)} px.
  </p>
  {#if activeHint}<p class="note">{activeHint}</p>{/if}
{:else}
  <p class="note bad">Проверьте значения: размеры и dpi больше нуля.</p>
{/if}

<div class="actions">
  <button class="ghost" disabled={!differs} onclick={reset}>Сбросить</button>
  <button class="accent" disabled={!differs || !valid} onclick={apply}>Применить</button>
</div>

<style>
  .presets {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
    margin-bottom: 14px;
  }
  .presets.fs {
    margin-top: -8px; /* ряд Фотосферы — продолжение общего */
  }
  .cap {
    align-self: center;
    font-size: 12px;
    color: var(--muted);
  }
  .presets button {
    padding: 0 10px;
    font-size: 13px;
  }
  .presets .active {
    background: var(--accent);
    color: #222;
  }
  .form {
    display: grid;
    gap: 8px;
  }
  label {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 8px;
    font-size: 13px;
  }
  label input[type='number'] {
    width: 90px;
  }
  .pair {
    display: flex;
    align-items: center;
    gap: 6px;
  }
  .pair input[type='number'] {
    width: 70px;
  }
  .check {
    justify-content: flex-start;
  }
  .note {
    font-size: 12px;
    color: var(--muted);
    margin: 12px 0 0;
  }
  .note.bad {
    color: #e0806f;
  }
  .actions {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
    margin-top: 12px;
  }
</style>
