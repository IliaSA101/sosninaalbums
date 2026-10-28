<script lang="ts">
  import { store } from '../../base/core/store.svelte'
  import { prefs } from '../../base/prefs/prefs.svelte'
  import { articleText } from '../guidetext/articletext'
  import { shortOf } from '../../base/prefs/levelcustom'
  import { LEVEL_TITLE, minLevel, rowOf, type Base } from '../../base/prefs/levels'
  import { articleById, placesOf, type Article } from '../guidetext/articles'
  import { guide } from './guidestate.svelte'
  import GuideVideo from './GuideVideo.svelte'
  import { learned } from './learned.svelte'
  import { closedLine, closedPlaces, keyRows, levelNeed, menuPath, openPlaces, planShow } from './show'

  // Карточка функции (спека 033): что делает, где, клавиши, уровень и «Показать». Тексты — только текстом.
  let { a }: { a: Article } = $props()

  const ctx = $derived(guide.ctx())
  const plan = $derived(planShow(a, ctx))
  const need = $derived(levelNeed(a, ctx))
  const hidden = $derived(need.steps.length > 0)
  const menus = $derived((a.menu ?? []).map(menuPath).filter((p): p is string => !!p))
  const keys = $derived(keyRows(a))
  // Кнопки — места, доступные на уровне; места выше уровня — серой строкой без кнопки (решение 133).
  const places = $derived(openPlaces(a, ctx))
  const later = $derived(closedPlaces(a, ctx))
  // Абзацы — куски текста по «\n»; разметки нет, всё текстовыми узлами.
  const paras = (t: string): string[] => articleText(t).split('\n')
  // «См. также»: только существующие статьи (страж текстов следит за id, здесь — защита от пустой ссылки).
  const see = $derived((a.see ?? []).map((id) => articleById(id)).filter((x): x is Article => !!x))
  const noPlace = $derived(placesOf(a).length === 0)

  // Строка про уровень (0.29-136-8): с какого уровня функция есть; скрытая — что включит «Показать».
  const FROM: Record<Base, string> = { simple: 'Простой и выше', standard: 'Стандартный и выше', full: 'Полный' }
  const levelLine = $derived.by(() => {
    if (!hidden) return FROM[minLevel(a.feature)]
    if (prefs.level === 'custom') return `Скрыто в вашем наборе. «Показать» добавит: ${need.added.map(shortOf).join(', ')}`
    return `Скрыто на уровне «${LEVEL_TITLE[prefs.level]}». «Показать» включит «${LEVEL_TITLE[need.to ?? 'standard']}»`
  })

  // Только эта функция — в «Свой» (решение 84), без смены готового уровня целиком.
  function onlyThis() {
    const e = prefs.toggle(rowOf(a.feature), true)
    if (e.added.length === 0) return
    store.notify(`Включено в «Свой»: ${e.added.map(shortOf).join(', ')}`, 'info', { label: 'Вернуть', run: () => void prefs.apply(e.prev) })
  }
</script>

<article class="card" aria-label={a.title} tabindex="-1">
  <h2>{a.title}</h2>
  <p class="what">{articleText(a.what)}</p>
  {#if a.where}<div class="row">
      <b>Где</b>
      <div class="paras">
        {#each paras(a.where) as t, i (i)}<p>{t}</p>{/each}
      </div>
    </div>{/if}
  {#each menus as m (m)}<div class="row"><b>Меню</b><span>{m}</span></div>{/each}
  {#if a.how}<div class="row">
      <b>Как</b>
      <div class="paras">
        {#each paras(a.how) as t, i (i)}<p>{t}</p>{/each}
      </div>
    </div>{/if}
  {#if keys.length > 0}
    <div class="row">
      <b>Клавиши</b>
      <span class="keys">
        {#each keys as k (k.show)}<span><kbd>{k.show}</kbd> — {k.label}</span>{/each}
      </span>
    </div>
  {/if}
  <div class="row" class:warn={hidden}>
    <b>Уровень</b>
    <span>
      {levelLine}
      {#if hidden && prefs.level !== 'custom'}
        <button class="link" onclick={onlyThis}>Только эту функцию — в «Свой»</button>
      {/if}
    </span>
  </div>
  {#if plan.note}<p class="note">{plan.note}</p>{/if}
  {#if noPlace}
    <p class="note">Показать на экране нечего — это жест или клавиша, см. «Как»</p>
  {:else if places.length > 0}
    <div class="act">
      {#each places as p, i (p.spot)}
        <button class:accent={i === 0} onclick={() => guide.show(a.id, p.spot)}>{p.label}</button>
      {/each}
    </div>
  {/if}
  {#each later as p (p.spot)}<p class="later">{closedLine(p, ctx)}</p>{/each}
  <GuideVideo id={a.id} title={a.title} />
  {#if see.length > 0}
    <div class="row">
      <b>См. также</b>
      <span class="see">
        {#each see as x (x.id)}<button class="link" onclick={() => (guide.sel = x.id)}>{x.title}</button>{/each}
      </span>
    </div>
  {/if}
  <!-- Состояние флажка — у Go: до ответа MarkGuide вид прежний; сам он не ставится ни выбором строки, ни «Показать» -->
  <label class="chk">
    <input
      type="checkbox"
      checked={learned.has(a.id)}
      onchange={(e) => {
        const on = e.currentTarget.checked
        e.currentTarget.checked = learned.has(a.id)
        void learned.set(a.id, on)
      }}
    />Изучил
  </label>
</article>

<style>
  .card {
    outline: none;
    display: flex;
    flex-direction: column;
    gap: 8px;
    line-height: 1.4;
  }
  h2 {
    margin: 0;
    font-size: 15px;
    font-weight: 600;
  }
  .what {
    margin: 0;
  }
  .row {
    display: grid;
    grid-template-columns: 72px 1fr;
    gap: 8px;
    color: var(--muted);
  }
  .row b {
    font-weight: 600;
    color: var(--text);
  }
  .row.warn span {
    color: var(--accent);
  }
  .paras {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }
  .paras p {
    margin: 0;
  }
  .see {
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    gap: 2px;
  }
  .keys {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }
  kbd {
    font: inherit;
    font-weight: 600;
    color: var(--text);
  }
  .note {
    margin: 0;
    color: var(--muted);
    font-style: italic;
  }
  .act {
    margin-top: 4px;
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
  }
  .later {
    margin: 0;
    color: var(--muted);
  }
  .chk {
    display: flex;
    align-items: center;
    gap: 6px;
    margin-top: 4px;
    align-self: flex-start;
  }
  .warn .link {
    margin-left: 6px;
  }
  .link {
    height: auto;
    padding: 0;
    background: transparent;
    color: var(--accent);
    white-space: normal;
    text-align: left;
  }
</style>
