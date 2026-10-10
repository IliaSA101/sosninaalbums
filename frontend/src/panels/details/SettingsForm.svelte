<script lang="ts">
  import { untrack } from 'svelte'
  import { store } from '../../base/core/store.svelte'
  import type { Settings } from '../../base/core/types'
  import ColorPicker from '../../base/ui/ColorPicker.svelte'
  import { presets, presetActive, presetFields, exportPx, type Preset } from './presets'

  const NUMS = ['pageW', 'pageH', 'margin', 'gap', 'dpi'] as const

  // Поле number при пустом вводе даёт null — приводим к числу при сравнении и сохранении.
  const norm = (x: Settings): Settings => ({
    pageW: Number(x.pageW),
    pageH: Number(x.pageH),
    margin: Number(x.margin),
    gap: Number(x.gap),
    dpi: Number(x.dpi),
    layflat: !!x.layflat,
    bg: x.bg ?? '', // "" — белый
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
    return n.pageW > 0 && n.pageH > 0 && n.dpi > 0 && n.margin >= 0 && n.gap >= 0 && NUMS.every((k) => Number.isFinite(n[k]))
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

  const activeHint = $derived(presets.find((p) => p.hint && presetActive(p, s))?.hint)

  function reset() {
    s = { ...store.view.settings }
    base = { ...store.view.settings }
  }
</script>

<div class="presets">
  {#each presets as p (p.label)}
    <button class:active={presetActive(p, s)} onclick={() => pick(p)}>{p.label}</button>
  {/each}
</div>

<div class="form">
  <label>Ширина страницы, мм <input type="number" min="100" max="600" bind:value={s.pageW} /></label>
  <label>Высота страницы, мм <input type="number" min="100" max="600" bind:value={s.pageH} /></label>
  <label>Поля, мм <input type="number" min="0" max="60" step="0.5" bind:value={s.margin} /></label>
  <label>Промежуток между фото, мм <input type="number" min="0" max="30" step="0.5" bind:value={s.gap} /></label>
  <label>Разрешение экспорта, dpi <input type="number" min="150" max="600" step="50" bind:value={s.dpi} /></label>
  <label>
    Цвет страниц альбома
    <!-- цвет попадает в форму при выборе; применяет «Применить» -->
    <ColorPicker value={s.bg || '#ffffff'} label="Цвет страниц альбома" onpick={(c) => (s.bg = c)} />
  </label>
  <label class="check">
    <input type="checkbox" bind:checked={s.layflat} />
    Фото может проходить через корешок (layflat-альбом)
  </label>
</div>
{#if valid}
  <p class="note">
    Разворот = две страницы: {(+s.pageW * 2) / 10} × {+s.pageH / 10} см, при экспорте {exportPx(+s.pageW * 2, +s.dpi)} ×
    {exportPx(+s.pageH, +s.dpi)} px.
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
