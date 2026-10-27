<script lang="ts">
  import { prefs } from '../../base/prefs/prefs.svelte'
  import { articleText } from '../guidetext/articletext'
  import { FINAL, INTRO } from './steps'
  import { cardPlace, introText, withMissing } from './tour'
  import { tour } from './tourstate.svelte'

  // Карточка тура (спека 113, design §4.4): приветствие со списком шагов, шаг, финал. Клавиши — только при фокусе
  // в карточке (keydown на ней, не на window): → и Enter дальше, ← назад, Esc закрыть.
  let winW = $state(0)
  let winH = $state(0)
  let w = $state(0)
  let h = $state(0)
  let primary = $state<HTMLElement>()
  let el = $state<HTMLElement>()

  const intro = $derived(tour.at === 'intro')
  const fin = $derived(tour.at === 'final')
  const step = $derived(tour.step)
  const num = $derived(typeof tour.at === 'number' ? tour.at + 1 : 0)
  const last = $derived(num === tour.steps.length)
  const at = $derived(cardPlace(tour.ring, { w, h }, { w: winW, h: winH }, tour.avoid))
  const text = $derived(tour.res ? withMissing(tour.res.text, tour.missing) : '')

  // Размер карточки — по границам (offset, не client: рамка 1 px). Меряем после отрисовки нового текста, до кадра:
  // место считается по высоте текущего шага, а не прошлого (ResizeObserver приходит на кадр позже).
  function measure() {
    if (!el) return
    w = el.offsetWidth
    h = el.offsetHeight
  }
  $effect(() => {
    void [tour.at, text, intro, fin, winW, winH]
    measure()
  })
  // Остальные причины смены размера (шрифт догрузился, окно сузило карточку).
  $effect(() => {
    if (!el || typeof ResizeObserver === 'undefined') return
    const ro = new ResizeObserver(measure)
    ro.observe(el)
    return () => ro.disconnect()
  })

  // После смены шага и при возвращении карточки (окно закрылось) фокус — на главной кнопке.
  $effect(() => {
    void tour.at
    primary?.focus()
  })

  function key(e: KeyboardEvent) {
    const t = e.target instanceof Element ? e.target : null
    if (e.key === 'Escape') tour.finish(fin ? 'done' : 'skip', 'key')
    else if (e.key === 'ArrowRight') tour.next()
    else if (e.key === 'ArrowLeft') tour.back()
    // Enter на «Назад», «Пропустить» и ссылках нажимает их самих
    else if (e.key === 'Enter' && !t?.closest('button:not([data-primary])')) tour.next()
    else {
      // Остальные клавиши (Del, Пробел…) в карточке программе не достаются; Ctrl-сочетания и Tab — как обычно.
      if (!e.ctrlKey && !e.metaKey && !e.altKey && e.key !== 'Tab') e.stopPropagation()
      return
    }
    e.preventDefault()
    e.stopPropagation()
  }
</script>

<svelte:window bind:innerWidth={winW} bind:innerHeight={winH} />

<div
  class="card"
  class:intro
  role="dialog"
  tabindex="-1"
  aria-labelledby="tour-title"
  data-tour-step={intro ? 'intro' : fin ? 'final' : step?.id}
  bind:this={el}
  style:left="{at.left}px"
  style:top="{at.top}px"
  style:opacity={h ? undefined : 0}
  onkeydown={key}
>
  {#if intro}
    <h3 id="tour-title">{INTRO.title}</h3>
    <p>{introText(articleText(INTRO.text, prefs.can), tour.steps.length)}</p>
    <ol class="list">
      {#each tour.steps as s, i (s.id)}
        <li><button class="link" onclick={() => tour.go(i)}>{i + 1}. {s.title}</button></li>
      {/each}
    </ol>
    <div class="btns">
      <button class="link" onclick={() => tour.finish('skip')}>Пропустить</button>
      <span class="grow"></span>
      <button onclick={() => tour.video()}>Видео о программе ↗</button>
      <button class="accent" data-primary bind:this={primary} onclick={() => tour.next()}>Начать</button>
    </div>
  {:else if fin}
    <h3 id="tour-title">{FINAL.title}</h3>
    <p>{articleText(FINAL.text, prefs.can)}</p>
    <div class="btns">
      <span class="grow"></span>
      <button onclick={() => tour.video()}>Видео о программе ↗</button>
      <button class="accent" data-primary bind:this={primary} onclick={() => tour.next()}>Закрыть</button>
    </div>
  {:else if step}
    <span class="cnt" id="tour-title">Шаг {num} из {tour.steps.length} · {step.title}</span>
    <p>{text}</p>
    <button class="link more" onclick={() => tour.openArticle(step.article)}>Подробнее в справочнике</button>
    <div class="btns">
      <button class="link" onclick={() => tour.finish('skip')}>Пропустить</button>
      <span class="grow"></span>
      {#if num > 1}<button onclick={() => tour.back()}>Назад</button>{/if}
      <button class="accent" data-primary bind:this={primary} onclick={() => tour.next()}>{last ? 'Готово' : 'Дальше'}</button>
    </div>
  {/if}
</div>

<style>
  .card {
    position: fixed;
    z-index: 91;
    width: 320px;
    max-width: calc(100vw - 16px);
    padding: 12px 14px 10px;
    display: flex;
    flex-direction: column;
    gap: 6px;
    background: var(--panel);
    border: 1px solid var(--line);
    border-radius: 8px;
    box-shadow: 0 16px 48px rgba(0, 0, 0, 0.55);
    font-size: 13px;
    outline: none;
  }
  .card.intro {
    width: 340px;
  }
  .cnt {
    color: var(--muted);
    font-size: 11px;
  }
  h3 {
    margin: 0;
    font-size: 14px;
  }
  p {
    margin: 0;
    line-height: 1.4;
    color: var(--text);
  }
  .list {
    margin: 2px 0;
    padding: 0;
    list-style: none;
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 0 8px;
  }
  .list button,
  .more {
    height: 22px;
    font-size: 12px;
    text-align: left;
  }
  .more {
    align-self: flex-start;
  }
  .btns {
    display: flex;
    align-items: center;
    gap: 6px;
    margin-top: 4px;
  }
  .btns button {
    height: 26px;
    font-size: 12px;
  }
  .grow {
    flex: 1;
  }
  .link {
    padding: 0 2px;
    background: transparent;
    color: var(--accent);
  }
  .link:hover {
    text-decoration: underline;
    background: transparent;
  }
</style>
