<script lang="ts">
  import { screenDpr } from '../../base/ui/dpr.svelte'
  import { Api, store } from '../../base/core/store.svelte'
  import type { PatternSpec, PatternView, SpreadView } from '../../base/core/types'
  import { isCover, ownBg } from '../../base/geom/sheet'
  import ColorPicker from '../../base/ui/ColorPicker.svelte'
  import { coverLabels } from '../cover/cover'
  import { SCOPE_LABEL } from '../backdrop/backdrop'
  import { decor } from '../../panels/decor/decor.svelte'
  import { presetOfView } from '../../panels/decor/decor'
  import { fadePct, pctFade } from '../backdrop/fade'
  import { patFadeLive, patScaleLive } from '../backdrop/fade.svelte'
  import {
    PATTERN_NEWER,
    PatSession,
    SCALE_MAX,
    SCALE_MIN,
    inksOf,
    newSeed,
    patKey,
    patched,
    patternCaption,
    patternPx,
    patternSrc,
    pctScale,
    scalePct,
  } from './pattern'

  // Строка «Узор» в разделе «Фон» (048, сц. 4): плитка и название, цвета, «Масштаб», «Прозрачность», «Перемешать», «Убрать».
  // Показывается, если узор есть. Ползунки — как «Прозрачность» фото фона: живой просмотр на oninput, один вызов Go
  // на change, стрелки в одном фокусе сливаются в один шаг отмены (merge-сессия). Цвет — один шаг на выбор.
  let { sp }: { sp: SpreadView } = $props()

  const list = $derived(ownBg(sp.patterns)) // фон альбома — не узор листа (098)
  const labels = $derived(isCover(sp) ? coverLabels(store.cover) : SCOPE_LABEL)
  const dpr = $derived(screenDpr.value)

  const set = (p: PatternView, patch: Partial<PatternSpec>, merge = '') =>
    store.run(Api.SetPattern(sp.id, p.scope, patched(p, patch), merge) as Promise<unknown>)
  const setInk = (p: PatternView, i: number, hex: string) => set(p, { inks: inksOf(p).map((c, j) => (j === i ? hex : c)) })

  // Ползунки: одна сессия merge на фокус. Она держится, пока тот же узор той же области (patKey), а не тот же
  // объект sp: после каждого ответа Go приходит новый sp, и сессия, привязанная к нему, рвала бы серию стрелок.
  const session = new PatSession()
  const val = (e: Event) => +(e.currentTarget as HTMLInputElement).value

  // Chromium не шлёт change, если значение при отпускании равно исходному: тогда просмотр залип бы.
  function blur(live: typeof patFadeLive, same: boolean) {
    session.end()
    if (same) live.release()
  }

  // Плитка каталога, из которой поставлен узор (PatternView.preset от Go, запасной — по цветам): имя и светлая подложка миниатюры — как на плитке окна «Фоны».
  const preset = (p: PatternView) => presetOfView(p, decor.bgItems)
  const nameOf = (p: PatternView) => (p.known ? (preset(p)?.name ?? p.name) : PATTERN_NEWER)
  const pageBg = (p: PatternView) => preset(p)?.color || sp.bg[p.scope === 'right' ? 1 : 0] || '#fff'

  // Уход с листа: просмотр не остаётся на чужом. Зависит от id листа (строка), а не от объекта sp.
  const sheetId = $derived(sp.id)
  $effect(() => {
    void sheetId
    return () => {
      session.end()
      patFadeLive.release()
      patScaleLive.release()
    }
  })
</script>

{#each list as p (p.scope)}
  {@const fade = patFadeLive.of(sp.id, p.scope, p.pat.fade ?? 0)}
  {@const scale = patScaleLive.of(sp.id, p.scope, p.pat.scale ?? 1)}
  <div class="pat" class:newer={!p.known} role="group" aria-label={patternCaption(p.scope, list.length, labels)}>
    <div class="cap">{patternCaption(p.scope, list.length, labels)}</div>
    <div class="have">
      {#if p.known && p.url}
        <img src={patternSrc(p.url, patternPx(44, dpr))} style:background={pageBg(p)} alt="" draggable="false" />
      {:else}
        <span class="ph" aria-hidden="true"></span>
      {/if}
      <div class="info">
        <span class="name" title={nameOf(p)}>{nameOf(p)}</span>
      </div>
    </div>
    {#if p.known}
      {#if p.inks > 0}
        <div class="row">
          <span class="lbl">{p.inks > 1 ? 'Цвета' : 'Цвет'}</span>
          <div class="inks">
            {#each inksOf(p) as c, i (i)}
              <ColorPicker value={c} label="Цвет узора {i + 1}" onpick={(hex) => setInk(p, i, hex)} />
            {/each}
          </div>
        </div>
      {/if}
      <div class="row" title="Крупнее или мельче мотив">
        <span class="lbl">Масштаб</span>
        <input
          type="range"
          min={SCALE_MIN}
          max={SCALE_MAX}
          step="1"
          value={scalePct(scale)}
          aria-label="Масштаб узора, %"
          aria-valuetext="{scalePct(scale)} %"
          onfocus={() => session.begin(patKey(sp.id, p))}
          onblur={() => blur(patScaleLive, scalePct(scale) === scalePct(p.pat.scale))}
          oninput={(e) => patScaleLive.set(sp.id, p.scope, pctScale(val(e)))}
          onchange={(e) => {
            const merge = session.merge(patKey(sp.id, p))
            const n = pctScale(val(e))
            return patScaleLive.commit(() => set(p, { scale: n }, merge))
          }}
          onpointerup={(e) => e.currentTarget.blur()}
        />
        <span class="num">{scalePct(scale)} %</span>
      </div>
      <div class="row" title="Чем больше, тем бледнее узор: сквозь него проступает цвет страницы">
        <span class="lbl">Прозрачность</span>
        <input
          type="range"
          min="0"
          max="100"
          step="1"
          value={fadePct(fade)}
          aria-label="Прозрачность узора, %"
          aria-valuetext="{fadePct(fade)} %"
          onfocus={() => session.begin(patKey(sp.id, p))}
          onblur={() => blur(patFadeLive, fadePct(fade) === fadePct(p.pat.fade ?? 0))}
          oninput={(e) => patFadeLive.set(sp.id, p.scope, pctFade(val(e)))}
          onchange={(e) => {
            const merge = session.merge(patKey(sp.id, p))
            const f = pctFade(val(e))
            return patFadeLive.commit(() => set(p, { fade: f }, merge))
          }}
          onpointerup={(e) => e.currentTarget.blur()}
        />
        <span class="num">{fadePct(fade)} %</span>
      </div>
    {/if}
    <div class="btns">
      {#if p.known && p.random}
        <button title="Другая расстановка мотива" onclick={() => set(p, { seed: newSeed(p.pat.seed) })}>Перемешать</button>
      {/if}
      <button class="ghost" onclick={() => store.run(Api.ClearPattern(sp.id, p.scope))}>Убрать</button>
    </div>
  </div>
{/each}

<style>
  .pat {
    margin-top: 10px;
    padding: 6px;
    border: 1px solid var(--line);
    border-radius: 6px;
    font-size: 12px;
  }
  .pat.newer {
    border-style: dashed;
  }
  .cap {
    font-weight: 600;
  }
  .have {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-top: 6px;
  }
  .have img,
  .ph {
    width: 44px;
    height: 44px;
    flex: none;
    object-fit: cover;
    border-radius: 4px;
    background: #fff; /* пока цвет страницы не известен */
  }
  .ph {
    border: 1px dashed var(--line);
    background: transparent;
  }
  .info {
    min-width: 0;
    display: flex;
    flex-direction: column;
  }
  .name {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .newer .name {
    color: var(--muted);
    white-space: normal;
  }
  .row {
    display: flex;
    align-items: center;
    gap: 6px;
    margin-top: 6px;
  }
  .lbl {
    flex: none;
    min-width: 72px;
    color: #cfcfd4;
  }
  .inks {
    display: flex;
    gap: 6px;
  }
  input[type='range'] {
    flex: 1;
    min-width: 0;
    height: 16px;
    margin: 0;
    accent-color: var(--accent);
  }
  .num {
    flex: none;
    width: 34px;
    text-align: right;
    font-variant-numeric: tabular-nums;
  }
  .btns {
    display: flex;
    flex-wrap: wrap; /* узкая колонка: кнопки переносятся, а не сжимаются */
    gap: 6px;
    margin-top: 6px;
  }
  .btns button {
    flex: 1 0 auto;
    white-space: nowrap;
    height: 24px;
    padding: 0 6px;
    font-size: 12px;
  }
</style>
