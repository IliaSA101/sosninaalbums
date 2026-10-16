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
  import { activeFamily, familiesIn, shownTpls, type VigFamilyFilter } from '../../base/tpl/tplfilter'
  import GalleryHead, { galleryName } from '../gallery/GalleryHead.svelte'
  import VigCard from './VigCard.svelte'
  import { changeTpl, vigChoices } from './vigact'
  import { canEditOwn, ownLabel, tileForm, tileLines } from './vigown'
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
  const allBuiltin = $derived((items ?? []).filter((c) => !c.own))
  const mine = $derived((items ?? []).filter((c) => c.own))
  // Фильтр видов (049): над встроенными; текущий шаблон виден всегда, свои не фильтруются. Состояние — в gallery (Пробел)
  const families = $derived(familiesIn(allBuiltin))
  const fam = $derived(activeFamily(gallery.vigFamily, families))
  const famItems = $derived<{ id: VigFamilyFilter; label: string }[]>([{ id: 'all', label: 'Все' }, ...families])
  const builtin = $derived(shownTpls(allBuiltin, fam, cur.tpl))
  // Тексты пустого ряда и причина серого «Мои» (050)
  const TEXTS = {
    empty: { all: 'Нет шаблонов для таких чисел', builtin: 'Нет шаблонов для таких чисел', mine: 'Нет своих шаблонов виньетки' },
    off: { builtin: 'Нет шаблонов для таких чисел', mine: 'Нет своих шаблонов виньетки' },
  }
  const view = $derived(tplSources(builtin, mine, gallery.source, TEXTS))
  const tiles = $derived(view.parts.reduce((n, p) => n + p.list.length, 0))
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

  // Закрываем сразу, как у обычных раскладок; шаблон, что уже стоит, — просто закрытие.
  // Уезжающая галерея inert: повторный щелчок не читает её $derived (cur) — derived_inert.
  function pick(c: VigChoice) {
    if (!gallery.cur) return
    const form = cur
    gallery.close()
    if (c.id !== form.tpl && !c.over && !c.gone) void changeTpl(sp.id, form, items ?? [], c.id)
  }

  // Портрет в мм и «мелко» — мелкой строкой под плиткой, полный текст и пояснение — в title
  const tip = (c: VigChoice) => `${ownLabel(c)}: ${portraitText(c)}. Под превью — ширина портрета ученика`
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
  {/snippet}
</GalleryHead>

<TplRow count={tiles} empty={items ? view.empty : ''} watch={[items, gallery.source, fam]}>
  {#snippet side()}
    <SideList items={view.items} value={gallery.source} onpick={(x) => gallery.setSource(x)} label="Какие шаблоны показать" />
  {/snippet}
  {#each view.parts as p, pi (pi)}
    {#if p.sep}<TplSep label={p.sep} />{/if}
    {#each p.list as c (c.id)}
      {@const edit = canEditOwn(c, canTpl)}
      <TplItem
        name={c.own ? ownLabel(c) : undefined}
        note={tileLines(c).join(' · ')}
        tone={c.over ? 'over' : c.small ? 'small' : undefined}
        onrename={edit ? () => vigOwn.rename(c) : undefined}
        ondelete={edit ? () => vigOwn.del(c) : undefined}
      >
        <button
          class="tile"
          class:current={c.id === cur.tpl}
          disabled={c.over || c.gone}
          title={tip(c)}
          aria-label="{ownLabel(c)}, {portraitText(c)}"
          onclick={() => pick(c)}
        >
          <VigCard {c} w={cardW} h={H} form={tileForm(items ?? [], cur.tpl, c.id, cur)} />
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
