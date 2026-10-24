<script lang="ts" module>
  // Пресеты страницы Go отдаёт списком: читаем один раз за запуск и заново после print:changed (123 Б3; как coverAct.loadPresets).
  let loaded: PagePreset[] | null = null
  let loadedRev = 0
  // Номер последнего запроса: два print:changed подряд дают два PagePresets, поздний ответ первого затёр бы свежий.
  let loadSeq = 0
</script>

<script lang="ts">
  import { untrack } from 'svelte'
  import { Api, store } from '../../base/core/store.svelte'
  import type { PagePreset, PagePxView, Settings } from '../../base/core/types'
  import Seg from '../../base/tpl/Seg.svelte'
  import ColorPicker from '../../base/color/ColorPicker.svelte'
  import { colorLive } from '../../base/color/colorlive.svelte'
  import { SHEET_PRESETS, cmText, isSheet, newSheetSize, unitOf } from '../../base/geom/sheet'
  import { modes } from '../../base/prefs/view.svelte'
  import AlbumBgRow from './AlbumBgRow.svelte'
  import PagePxRow from './PagePxRow.svelte'
  import { PxAsker } from './pagepx'
  import MarkupBlock from '../markup/MarkupBlock.svelte'
  import { CROP_ITEMS, cropChangeNote, cropNote, cropOf } from './crop'
  import { FOLD_ITEMS, foldChangeNote, foldFields, foldNote, foldOf } from './fold'
  import { printEdit } from '../../shell/printprof/printedit.svelte'
  import { printList } from '../../shell/printprof/printlist.svelte'
  import { NEW_ITEM, NEW_VALUE } from '../../shell/printprof/printwords'
  import { groupLabel, presetActive, presetFields, presetGroupOf, presetHint, presetRows, exportPx, FILE_DPI, type Preset } from './presets'
  import { NUMS, norm, same } from './settingsnorm'
  import { marginNoteAsker } from './marginnote'
  import AlbumOwnAsk from '../albumbg/AlbumOwnAsk.svelte'
  import { ownAskText, ownColorNums, ownDoneText, type OwnAsk } from '../albumbg/albumown'

  // base — настройки, с которыми форма сверялась последний раз.
  let base = $state<Settings>({ ...store.view.settings })
  let s = $state<Settings>({ ...store.view.settings })

  const edited = $derived(!same(s, base))
  const differs = $derived(!same(s, store.view.settings))
  const valid = $derived.by(() => {
    const n = norm(s)
    return (
      n.pageW > 0 &&
      n.pageH > 0 &&
      n.dpi > 0 &&
      n.margin >= 0 &&
      n.gap >= 0 &&
      n.safe >= 0 &&
      (n.bleed ?? 0) >= 0 &&
      NUMS.every((k) => Number.isFinite(n[k]))
    )
  })

  // Настройки сменились не отсюда (Undo, второе окно): без своих правок форма подтягивает их.
  $effect(() => {
    const cur = store.view.settings
    untrack(() => {
      if (same(cur, base)) return
      // Список «Типография» следует за альбомом: ручной выбор до применения (picked) к чужой смене настроек не относится.
      if (!edited) {
        s = { ...cur }
        picked = null
      }
      base = { ...cur }
    })
  })

  // Один вызов — один шаг отмены; живое применение забило бы историю пересчётами.
  // 115: цвет альбома сменился, а у разворотов есть свой цвет — сначала вопрос (правки ещё нет); × оставляет черновик формы.
  let own = $state<{ ask: OwnAsk; n: Settings; count: number } | null>(null)
  const colorKey = (c: string | undefined) => (c || '#ffffff').toLowerCase()
  async function apply() {
    await pxAsk.settled() // ввод px ещё у Go — страница в форме должна успеть обновиться
    const n = norm(s)
    const nums = ownColorNums(store.view)
    if (nums.length > 0 && colorKey(n.bg) !== colorKey(store.view.settings.bg)) {
      own = { ask: ownAskText('color', { nums, photos: 0 }, unitOf(store.view.settings)), n, count: nums.length }
      return
    }
    await put(n)
  }
  // all — сколько разворотов станут «как в альбоме» (ответ «Как в альбоме»); 0 — прежний SetSettings («Оставить свой»).
  async function put(n: Settings, all = 0) {
    own = null
    if (all > 0) {
      if (await store.run(Api.SetSettingsToAll(n))) store.notify(ownDoneText('color', all, unitOf(store.view.settings)))
    } else await store.setSettings(n)
    s = { ...store.view.settings }
    base = { ...store.view.settings }
  }

  // Файл разворота в px (123): страницу по введённым px считает Go, форма только принимает её.
  const pxAsk = new PxAsker()
  function fillPage(r: PagePxView) {
    s = { ...s, pageW: r.pageW, pageH: r.pageH }
  }

  // Пресет только заполняет форму; применяет «Применить» — один шаг undo.
  function pick(p: Preset) {
    s = { ...s, ...presetFields(p) }
  }

  // Лист виньетки (031): размер листа вместо страницы, пресеты 20×30 и 30×40, layflat и «Свой» не нужны.
  const sheet = $derived(isSheet(store.view.settings))
  let presets = $state<PagePreset[]>(loaded ?? [])
  const allRows = $derived(presetRows(presets))
  // Список «Типография» (058): выбранное вручную главнее; иначе группа подсвеченного пресета или последняя выбранная.
  let picked = $state<string | null>(null)
  // Выбранной вручную типографии могло не стать (удалена своя): тогда по пресету.
  const group = $derived(picked !== null && allRows.some((r) => r.group === picked) ? picked : presetGroupOf(presets, s, modes.presetGroup))
  const rows = $derived(sheet ? [{ group: '', items: [...SHEET_PRESETS] }] : allRows.filter((r) => r.group === group))
  // Последний пункт списка — «Своя типография…»: окно на основе выбранной типографии; список остаётся на ней.
  function pickGroup(g: string, el: HTMLSelectElement) {
    if (g === NEW_VALUE) {
      el.value = group
      void printEdit.show(printList.byName(group)?.id ?? '')
      return
    }
    picked = g
    modes.setPresetGroup(g)
  }
  // Сохранённая в окне своя типография поставлена альбому: список встаёт на неё по форме, а не по прежнему выбору.
  $effect(() => {
    void printList.saves
    picked = null
  })

  // Не удалось получить — формы это не мешает: остаётся ввод руками. Реестр сменился (print:changed) — читаем заново.
  $effect(() => {
    const rev = printList.rev
    if (loaded && loadedRev === rev) return
    loadedRev = rev
    const my = ++loadSeq
    Api.PagePresets()
      .then((l) => {
        if (my === loadSeq) presets = loaded = (l ?? []) as PagePreset[]
      })
      .catch(() => {})
  })

  const activeHint = $derived(presetHint(presets, s, group))

  // Поля у линии реза (0.23-q7 а): текст считает Go по черновику; пусто — строки нет.
  let marginText = $state('')
  const marginNote = marginNoteAsker(
    (m, b) => Api.MarginNote(m, b),
    (t) => (marginText = t),
  )
  $effect(() => marginNote.ask(s.margin, s.bleed ?? 0))

  function reset() {
    s = { ...store.view.settings }
    base = { ...store.view.settings }
  }
</script>

{#if !sheet && allRows.length > 0}
  <div class="group" data-guide="fsfera-presets">
    <label class="cap" for="preset-group">Типография</label>
    <select id="preset-group" value={group} onchange={(e) => pickGroup(e.currentTarget.value, e.currentTarget)}>
      {#each allRows as r (r.group)}<option value={r.group}>{r.group ? printList.label(r.group) : groupLabel(r.group)}</option>{/each}
      <!-- черта перед «Своей типографией…» (макет 123 Б3) -->
      <hr />
      <option value={NEW_VALUE}>{NEW_ITEM}</option>
    </select>
  </div>
{/if}
{#each rows as row (row.group)}
  <div class="presets">
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
  <label title="Срежется при обрезке; полоса — Вид → Безопасная зона"
    >Обрез, мм <input type="number" min="0" max="10" step="0.5" bind:value={() => s.bleed ?? 0, (v) => (s.bleed = v)} /></label
  >
  <label>Промежуток между фото, мм <input type="number" min="0" max="30" step="0.5" bind:value={s.gap} /></label>
  <label>Разрешение экспорта, dpi <input type="number" min="150" max="600" step="50" bind:value={s.dpi} /></label>
  <label>
    {sheet ? 'Цвет листов' : 'Цвет страниц альбома'}
    <!-- цвет попадает в форму при выборе; применяет «Применить» -->
    <!-- наведение показывает цвет на страницах без своего цвета (109); выбор остаётся в черновике до «Применить» -->
    <ColorPicker
      value={s.bg || '#ffffff'}
      sheet={store.spread?.id}
      label={sheet ? 'Цвет листов' : 'Цвет страниц альбома'}
      onpick={(c) => (s.bg = c)}
      onpreview={(c) => (c ? colorLive.set(['album'], c) : colorLive.clear())}
    />
  </label>
  <!-- 098: фон альбома — не поле формы, ставится сразу (свой шаг отмены); цвет страниц лежит под ним -->
  <AlbumBgRow />
  {#if !sheet}
    <!-- Положение — в черновике формы; Go применит «Применить» (SetSettings, один шаг undo) -->
    <div class="fold" data-guide="fold-mode">
      <span class="cap" title="Для layflat-альбомов: разворот раскрывается плоско">Фото через корешок</span>
      <div class="foldseg">
        <Seg look="card" label="Фото через корешок" items={FOLD_ITEMS} value={foldOf(s)} onpick={(m) => Object.assign(s, foldFields(m))} />
      </div>
      <p class="note">{foldNote(foldOf(s))}</p>
      {#if foldChangeNote(foldOf(store.view.settings), foldOf(s))}
        <p class="note change">{foldChangeNote(foldOf(store.view.settings), foldOf(s))}</p>
      {/if}
    </div>
    <!-- 104: тот же вид, что у «Фото через корешок»; предел подрезки — в черновике формы, применяет «Применить» -->
    <div class="fold" data-guide="crop-mode">
      <span class="cap" title="Фото чуть обрезаются по краям, чтобы раскладка дошла до полей">Подрезать фото, чтобы закрыть лист</span>
      <div class="foldseg">
        <Seg look="card" label="Подрезать фото, чтобы закрыть лист" items={CROP_ITEMS} value={cropOf(s)} onpick={(c) => (s.crop = c)} />
      </div>
      <p class="note">{cropNote(cropOf(s))}</p>
      {#if cropChangeNote(cropOf(store.view.settings), cropOf(s))}
        <p class="note change">{cropChangeNote(cropOf(store.view.settings), cropOf(s))}</p>
      {/if}
    </div>
  {/if}
</div>
{#if valid}
  {@const sh = newSheetSize({ pageW: +s.pageW, pageH: +s.pageH, sheet: s.sheet })}
  <p class="note">
    {sh.mid > 0 ? 'Разворот = две страницы' : 'Лист'}: {cmText(sh.w)} × {cmText(sh.h)} см, при экспорте {exportPx(sh.w, +s.dpi)} ×
    {exportPx(sh.h, +s.dpi)} px.
  </p>
  {#if !sheet}
    <PagePxRow px={exportPx(sh.w, FILE_DPI)} py={exportPx(sh.h, FILE_DPI)} asker={pxAsk} onfill={fillPage} />
  {/if}
  {#if activeHint}<p class="note">{activeHint}</p>{/if}
  {#if marginText}<p class="note">{marginText}</p>{/if}
{:else}
  <p class="note bad">Проверьте значения: размеры и dpi больше нуля.</p>
{/if}

<div class="actions">
  <button class="ghost" disabled={!differs} onclick={reset}>Сбросить</button>
  <button class="accent" disabled={!differs || !valid} onclick={apply}>Применить</button>
</div>

{#if own}
  {@const o = own}
  <AlbumOwnAsk ask={o.ask} onkeep={() => put(o.n)} onall={() => put(o.n, o.count)} onclose={() => (own = null)} />
{/if}

<!-- Не часть формы: файл выбирается сразу, один шаг отмены (066) -->
<div class="markup">
  <MarkupBlock kind="spreads" guide={store.view.spreadMarkup ?? null} />
  <p class="note">Файл разметки действует сразу, без «Применить».</p>
</div>

<style>
  .presets {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
    margin-bottom: 14px;
  }
  .group {
    display: flex;
    flex-direction: column;
    gap: 4px;
    margin-bottom: 8px;
  }
  .cap {
    font-size: 12px;
    font-weight: 600;
  }
  .group select {
    width: 100%;
    height: 24px;
    padding: 0 4px;
    font-size: 12px;
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
  .fold .cap {
    display: block;
  }
  .foldseg {
    margin-top: 6px;
  }
  .fold .note {
    margin-top: 8px;
    line-height: 1.4;
  }
  .fold .note.change {
    color: #cfcfd4;
  }
  .note {
    font-size: 12px;
    color: var(--muted);
    margin: 12px 0 0;
  }
  .note.bad {
    color: #e0806f;
  }
  .markup {
    margin-top: 14px;
    padding-top: 6px;
    border-top: 1px solid var(--line);
  }
  .markup .note {
    margin: 4px 0 0;
  }
  .actions {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
    margin-top: 12px;
  }
</style>
