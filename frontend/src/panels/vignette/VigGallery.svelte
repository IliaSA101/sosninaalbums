<script lang="ts">
  import { store } from '../../base/core/store.svelte'
  import { errText } from '../../base/words/errtext'
  import { sheetSize } from '../../base/geom/sheet'
  import type { SpreadView, VigChoice, VignetteView } from '../../base/core/types'
  import { gallery } from '../gallery/gallery.svelte'
  import { prefs } from '../../base/prefs/prefs.svelte'
  import TplRow from '../../base/tpl/TplRow.svelte'
  import TplItem from '../../base/tpl/TplItem.svelte'
  import TplSep from '../../base/tpl/TplSep.svelte'
  import SideList from '../../base/tpl/SideList.svelte'
  import Seg from '../../base/tpl/Seg.svelte'
  import { tplSources } from '../../base/tpl/tplsource'
  import { activeFamily, familiesIn, type VigFamilyFilter } from '../../base/tpl/tplfilter'
  import GalleryHead, { galleryName } from '../gallery/GalleryHead.svelte'
  import VigCard from './VigCard.svelte'
  import { changeTpl, vigChoices } from './vigact'
  import { FORMS, FORM_FILTERS_SHORT, curKey, pickTile, tileFormOf, vigTiles, type Tile } from './vigform'
  import { canEditOwn, ownLabel, tileLines } from './vigown'
  import { vigOwn } from './vigown.svelte'
  import { formOf, portraitText } from './vignette'

  // Галерея ▦ на виньетке (решение 80): та же рамка, что у страницы (GridGallery) — шапка (GalleryHead), один ряд
  // плиток (TplRow/TplItem) и переключатель «Все / Встроенные / Мои» слева (050); «Сохранить как шаблон…» в шапке. Числа и пропорция — те, что стоят
  // на листе; щелчок применяет шаблон (один шаг отмены), на ручной виньетке — после вопроса, как у страницы.
  let { sp }: { sp: SpreadView & { vig: VignetteView } } = $props()

  const H = 80 // высота превью, как у карточек сеток страницы
  const sheet = $derived(sheetSize(sp))
  const cardW = $derived((H * sheet.w) / sheet.h)
  const cur = $derived(formOf(sp.vig))
  const canTpl = $derived(prefs.can('templates')) // сохранить, ✎ и × — с уровня «Шаблоны»

  let items = $state<VigChoice[] | null>(null) // null — ответ Go ещё не пришёл
  const catalog = $derived((items ?? []).filter((c) => !c.own))
  // Фильтры вида (049) и формы (056): над встроенными; текущая плитка видна всегда, свои вид не фильтрует. Состояние —
  // в gallery (Пробел листает то же). Плитка = шаблон + форма; на «Мои» фильтр формы серый и свои показаны все.
  const families = $derived(familiesIn(catalog))
  const fam = $derived(activeFamily(gallery.vigFamily, families))
  const famItems = $derived<{ id: VigFamilyFilter; label: string }[]>([{ id: 'all', label: 'Все' }, ...families])
  const all = $derived(vigTiles(items ?? [], fam, gallery.source === 'mine' ? 'all' : gallery.vigForm, cur))
  const builtin = $derived(all.filter((t) => !t.c.own))
  const mine = $derived(all.filter((t) => t.c.own))
  const curK = $derived(curKey(all, cur))
  // Тексты пустого ряда и причина серого «Мои» (050)
  const TEXTS = {
    empty: { all: 'Нет шаблонов для таких чисел', builtin: 'Нет шаблонов для таких чисел', mine: 'Нет своих шаблонов виньетки' },
    off: { builtin: 'Нет шаблонов для таких чисел', mine: 'Нет своих шаблонов виньетки' },
  }
  const view = $derived(tplSources(builtin, mine, gallery.source, TEXTS))
  const count = $derived(view.parts.reduce((n, p) => n + p.list.length, 0))
  let req = 0
  $effect(() => {
    const f = { teachers: cur.teachers, pupils: cur.pupils, ratio: cur.ratio }
    void store.view.rev
    void vigOwn.rev // свои шаблоны сохранили, переименовали или удалили
    const my = ++req
    vigChoices(sp.id, f)
      .then((list) => {
        if (my === req) items = list
      })
      .catch((e) => {
        if (my === req) store.notify(errText(e), 'error')
      })
  })

  // Закрываем сразу, как у обычных раскладок; плитка, что уже стоит, — просто закрытие. Остальные ставят шаблон и свою
  // форму (один шаг отмены). Уезжающая галерея inert: повторный щелчок не читает её $derived (cur) — derived_inert.
  function pick(t: Tile) {
    if (!gallery.cur) return
    const form = cur
    const same = t.key === curK
    gallery.close()
    if (!same && !t.c.over && !t.c.gone) void changeTpl(sp.id, form, pickTile(t, form.ratio))
  }

  // Портрет в мм и «мелко» — мелкой строкой под плиткой, полный текст и пояснение — в title
  const tip = (t: Tile) => `${ownLabel(t.c)}, ${FORMS[t.k].name}: ${portraitText(t.c)}. Под превью — ширина портрета ученика`
</script>

<GalleryHead
  tabs={false}
  title={galleryName(sp.id)}
  save={canTpl
    ? { title: 'Запомнить места, шапку и стиль подписей этого листа как свой шаблон', onclick: () => vigOwn.save(sp.id) }
    : undefined}
>
  {#snippet filters()}
    {#if famItems.length > 2}
      <!-- На «Мои» сегменты серые, а не пропадают: шапка не прыгает (решение №158) -->
      <Seg
        items={famItems}
        value={fam}
        onpick={(f) => gallery.setVigFamily(f)}
        label="Вид шаблонов"
        off={gallery.source === 'mine'}
        offTitle="Свои шаблоны не делятся по видам"
      />
    {/if}
    {#if catalog.length > 0}
      <Seg
        items={FORM_FILTERS_SHORT}
        value={gallery.vigForm}
        onpick={(f) => gallery.setVigForm(f)}
        label="Форма мест"
        off={gallery.source === 'mine'}
        offTitle="Свои шаблоны показаны в своей форме"
      />
    {/if}
  {/snippet}
</GalleryHead>

<TplRow {count} empty={items ? view.empty : ''} watch={[items, gallery.source, fam, gallery.vigForm]}>
  {#snippet side()}
    <SideList items={view.items} value={gallery.source} onpick={(x) => gallery.setSource(x)} label="Какие шаблоны показать" />
  {/snippet}
  {#each view.parts as p, pi (pi)}
    {#if p.sep}<TplSep label={p.sep} />{/if}
    {#each p.list as t (t.key)}
      {@const c = t.c}
      {@const edit = canEditOwn(c, canTpl)}
      <!-- Группа формы — тонкая черта со значком (полное слово в подсказке); у своих черта «мои» уже есть -->
      {#if t.sep && !c.own}<TplSep label={FORMS[t.k].glyph} title={t.sep} />{/if}
      <TplItem
        name={c.own ? ownLabel(c) : undefined}
        note={tileLines(c).join(' · ')}
        tone={c.over ? 'over' : c.small ? 'small' : undefined}
        onrename={edit ? () => vigOwn.rename(c) : undefined}
        ondelete={edit ? () => vigOwn.del(c) : undefined}
      >
        <button
          class="tile"
          class:current={t.key === curK}
          disabled={c.over || c.gone}
          title={tip(t)}
          aria-label="{ownLabel(c)}, {FORMS[t.k].name}, {portraitText(c)}"
          onclick={() => pick(t)}
        >
          <VigCard {c} w={cardW} h={H} form={tileFormOf(t)} />
        </button>
      </TplItem>
    {/each}
  {/each}
</TplRow>

<style>
  .tile {
    all: unset;
    display: block;
    cursor: pointer;
  }
  .tile:disabled {
    cursor: default;
  }
  /* Рамка — вокруг превью */
  .tile :global(.card) {
    outline: 2px solid transparent;
    outline-offset: 2px;
    transition: outline-color 0.15s;
  }
  .tile:hover:not(:disabled) :global(.card) {
    outline-color: #8a8a90;
  }
  .tile.current :global(.card) {
    outline-color: var(--accent);
  }
  .tile:focus-visible :global(.card) {
    outline-color: var(--accent);
  }
</style>
