<script lang="ts">
  import { Api, store } from '../../base/core/store.svelte'
  import type { SpreadView } from '../../base/core/types'
  import { prefs } from '../../base/prefs/prefs.svelte'
  import ColorPicker from '../../base/color/ColorPicker.svelte'
  import { colorLive } from '../../base/color/colorlive.svelte'
  import InspectorBackdrop from '../backdrop/InspectorBackdrop.svelte'
  import InspectorPattern from '../pattern/InspectorPattern.svelte'
  import { coverLabel } from '../cover/cover'
  import { isCover, isSheet, ownBg, unitOf } from '../../base/geom/sheet'
  import {
    ALBUM_CARD_EDIT,
    ALBUM_CARD_HIDE,
    ALBUM_CARD_NOW,
    ALBUM_CARD_OFF,
    ALBUM_CARD_OWN,
    ALBUM_CARD_SHOW,
    ALBUM_CARD_TO,
    ALBUM_CARD_TO_TIP,
    albumCard,
  } from '../../panels/albumbg/albumbg'
  import { decor } from '../../panels/decor/decor.svelte'
  import CardGroup from './CardGroup.svelte'
  import { bgKind, bgSum } from './groupsum'

  // Группа «Фон» карточки листа (009 D2, спека 124 U5): цвет на весь разворот или по страницам, фон альбома, узор
  // и зоны «Фото фона». Тело вынесено из InspectorSpread. dropOpen: пока тянут фото, свёрнутая группа раскрыта —
  // в ней зона броска. «×» у цвета возвращает фон альбома тем же SetBackground, что окно цвета (один шаг отмены).
  // Обложка (124 U6): группа «cover.bg» — цвет («По сторонкам») и узор; зоны фото фона у неё в группе «Фото фона».
  let { sp }: { sp: SpreadView } = $props()

  // Лист виньетки (031): страниц нет — фон только на весь лист, «По страницам» не предлагается.
  const sheet = $derived(isSheet(store.view.settings))
  const unit = $derived(unitOf(store.view.settings))
  // Обложка (032): названия — сторонки.
  const cover = $derived(isCover(sp))
  const sides = $derived(
    cover
      ? ([
          { scope: 'left', label: coverLabel(store.cover, 'left'), i: 0 },
          { scope: 'right', label: coverLabel(store.cover, 'right'), i: 1 },
        ] as const)
      : ([
          { scope: 'left', label: 'Левая', i: 0 },
          { scope: 'right', label: 'Правая', i: 1 },
        ] as const),
  )
  const allLabel = $derived(cover ? coverLabel(store.cover, 'spread') : unit.all)

  // «По страницам» раскрыто вручную; если цвета страниц уже разные, раскрыто всегда (один образец
  // соврал бы). Смена разворота сворачивает.
  let splitOpen = $state(false)
  let splitFor = ''
  $effect(() => {
    if (sp.id !== splitFor) {
      splitFor = sp.id
      splitOpen = false
    }
  })
  const differ = $derived(sp.bg[0] !== sp.bg[1])
  const split = $derived(!sheet && (splitOpen || differ))
  const own = $derived(sp.bgSet[0] !== '' || sp.bgSet[1] !== '')

  // Фон альбома (098): строка — виден ли он на этом листе и как его скрыть; сам фон выбирают в настройках альбома.
  const albumRow = $derived(albumCard(sp, store.view.albumBg))
  const hideAlbum = (off: boolean) => store.run(Api.SetNoAlbumBg(sp.id, off))
  // 115 Г2: свой узор и фото фона сняты, «Не показывать здесь» тоже — одна правка без вопроса (вернуть — Ctrl+Z)
  const toAlbum = () => store.run(Api.SpreadBgToAlbum(sp.id))
  // 115 В: строка «Как в альбоме» в окнах цвета — цвет альбома и «своего цвета нет» (у «всего разворота» — ни на одной странице)
  const albumHex = $derived(store.view.settings.bg || '#ffffff')
  // ColorPicker зовёт onpick один раз на выбор — один шаг undo. Цвет '' снимает свой цвет.
  // Показ держится до ответа Go (colorLive.commit): старый фон не мелькает.
  const pick = (scope: 'spread' | 'left' | 'right', color: string) =>
    colorLive.commit(() => store.run(Api.SetBackground(sp.id, scope, color)))
  // Наведение на цвет — фон листа или страницы сразу, без правки (109).
  const preview = (scope: 'spread' | 'left' | 'right', c: string | null) =>
    c ? colorLive.set([`bg:${sp.id}:${scope}`], c) : colorLive.clear()

  // Фото фона обложки в свёрнутой строке не пишется: оно в группе «Фото фона».
  const summary = $derived(
    bgSum(bgKind({ photo: !cover && ownBg(sp.backdrops).length > 0, pattern: ownBg(sp.patterns).length > 0, color: own })),
  )

  // Обложка без цвета на уровне и без лежащего узора: у группы нет ни одной строки — заголовок не рисуем (0.26-124-28, -39).
  const bare = $derived(cover && !prefs.can('bgColor') && ownBg(sp.patterns).length === 0)
</script>

{#if !bare}
  <CardGroup id={cover ? 'cover.bg' : 'spread.bg'} title="Фон" {summary} dropOpen={!cover}>
    <div role="group" aria-label="Фон" data-guide="spread-bg">
      {#if prefs.can('bgColor')}
        {#if !split}
          <div class="row">
            <span class="lbl">{allLabel}&#32;<span class="src">· {own ? 'свой' : 'как в альбоме'}</span></span>
            <ColorPicker
              value={sp.bg[0]}
              sheet={sp.id}
              label="Цвет фона {unit.one}а"
              inherit={{ hex: albumHex, on: !own }}
              oninherit={() => pick('spread', '')}
              onpick={(c) => pick('spread', c)}
              onpreview={(c) => preview('spread', c)}
            />
            <button
              class="reset"
              disabled={!own}
              aria-label="{allLabel}: как в альбоме"
              title="Как в альбоме"
              onclick={() => pick('spread', '')}>×</button
            >
          </div>
        {/if}
        {#if !sheet}
          <button class="link" aria-expanded={split} disabled={differ} onclick={() => (splitOpen = !splitOpen)}>
            <svg class="chev" class:open={split} viewBox="0 0 12 12" aria-hidden="true"><path d="M4 2.5 7.5 6 4 9.5" /></svg>
            {cover ? 'По сторонкам' : 'По страницам'}
          </button>
        {/if}
        {#if split}
          {#each sides as s (s.scope)}
            <div class="row">
              <span class="lbl">{s.label}&#32;<span class="src">· {sp.bgSet[s.i] === '' ? 'как в альбоме' : 'свой'}</span></span>
              <ColorPicker
                value={sp.bg[s.i]}
                sheet={sp.id}
                label="Цвет фона: {s.label.toLowerCase()} страница"
                inherit={{ hex: albumHex, on: sp.bgSet[s.i] === '' }}
                oninherit={() => pick(s.scope, '')}
                onpick={(c) => pick(s.scope, c)}
                onpreview={(c) => preview(s.scope, c)}
              />
              <button
                class="reset"
                disabled={sp.bgSet[s.i] === ''}
                aria-label="{s.label} страница: как в альбоме"
                title="Как в альбоме"
                onclick={() => pick(s.scope, '')}>×</button
              >
            </div>
          {/each}
        {/if}
      {/if}
      {#if albumRow}
        <div class="album">
          {#if albumRow === 'on'}
            <span class="lbl">{ALBUM_CARD_NOW}</span>
            <!-- Без уровня «Фоны» окно не открываем — как «Выбрать…» в AlbumBgRow -->
            {#if prefs.can('decor')}<button class="ghost mini" onclick={() => decor.showAlbum()}>{ALBUM_CARD_EDIT}</button>{/if}
            <button class="ghost mini" onclick={() => hideAlbum(true)}>{ALBUM_CARD_HIDE}</button>
          {:else if albumRow === 'off'}
            <span class="lbl">{ALBUM_CARD_OFF}</span>
            <button class="ghost mini" onclick={() => hideAlbum(false)}>{ALBUM_CARD_SHOW}</button>
          {:else}
            <span class="lbl">{ALBUM_CARD_OWN}</span>
            <button class="ghost mini" title={ALBUM_CARD_TO_TIP} onclick={toAlbum}>{ALBUM_CARD_TO}</button>
          {/if}
        </div>
      {/if}
      <!-- Узор программы (048) — строкой «Узор» над зонами: он лежит под фото фона и от областей зон не зависит -->
      {#if cover}
        {#if prefs.can('bgPhoto')}<InspectorPattern {sp} />{/if}
      {:else if prefs.can('bgPhoto')}
        <div data-slot="backdrop"><InspectorPattern {sp} /><InspectorBackdrop {sp} {split} /></div>
      {/if}
    </div>
  </CardGroup>
{/if}

<style>
  .row {
    display: flex;
    align-items: center;
    gap: 6px;
    margin-top: 6px;
    font-size: 12px;
  }
  .lbl {
    flex: 1;
    color: var(--muted);
  }
  .src {
    opacity: 0.7;
  }
  .link {
    all: unset;
    box-sizing: border-box;
    display: flex;
    align-items: center;
    gap: 4px;
    margin-top: 8px;
    font-size: 12px;
    color: var(--muted);
    cursor: pointer;
  }
  .link:hover:not(:disabled) {
    color: var(--text);
  }
  .link:disabled {
    cursor: default;
    opacity: 0.6;
  }
  .link:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: 1px;
  }
  .chev {
    width: 10px;
    height: 10px;
    fill: none;
    stroke: currentColor;
    stroke-width: 1.6;
    stroke-linecap: round;
    stroke-linejoin: round;
    transition: transform var(--move);
  }
  .chev.open {
    transform: rotate(90deg);
  }
  .album {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 4px 6px;
    margin-top: 8px;
    font-size: 12px;
  }
  .album .lbl {
    flex-basis: 100%;
  }
  .ghost {
    height: 24px;
    padding: 0 8px;
    font-size: 12px;
  }
  .reset {
    width: 22px;
    height: 24px;
    padding: 0;
    font-size: 14px;
    background: transparent;
    color: #cfcfd4;
    border-radius: 4px;
  }
</style>
