<script lang="ts">
  import { onMount, tick } from 'svelte'
  import { store } from '../../base/core/store.svelte'
  import { sessionId } from '../../editor/text/textedit'
  import { edit } from '../../editor/text/edit.svelte'
  import { PersonSend } from './personsend'
  import { personOf } from './names'
  import { cardOf, loadPerson, savePerson, type PersonTarget, type PlaceRef } from './personapi'
  import { useCard } from '../../base/prefs/cardgroups.svelte'
  import { nameSum } from '../../editor/inspector/groupsum'

  // Имя на фото (спека 030) в карточке фото: Фамилия, Имя, Отчество, у учителя виньетки ещё Предмет (049). Пока своего имени нет, значения берутся
  // из имени файла и видны серым («из имени файла»); все три поля очищены — снова имя из файла. Набор уходит в Go
  // с паузой, одной сессией на фокус (один шаг отмены), и при смене фото досылается.
  // place (071) — имя пустого места (табличка): те же поля, но через PlacePerson / SetPlacePerson; photo при этом ''.
  // У фото на месте с табличкой поля показывают её же («Имя места — останется здесь…»; у таблички только с предметом — «Предмет места…»).
  // inline — окошко имени на холсте (037): без заголовка, показаны только поля из only, фокус — в первом,
  // Enter ведёт к следующему полю (на последнем — ondone), вся сессия набора — один шаг отмены.
  // row (спека 124, 0.26-124-3): Фамилия, Имя, Отчество одной строкой, Предмет — строкой ниже. В узкой карточке (200 px) в полях
  // короткие подсказки «Фам.», «Имя», «Отч.» (полные слова — в aria-label), title поля — его значение, поле в фокусе шире соседних.
  // cap=false — подпись даёт группа карточки; onsummary — значение для свёрнутой группы («Иванов Иван» или «нет»).
  let {
    photo,
    place,
    only,
    inline = false,
    row = false,
    cap = true,
    onsummary,
    ondone,
  }: {
    photo: string
    place?: PlaceRef
    only?: readonly string[]
    inline?: boolean
    row?: boolean
    cap?: boolean
    onsummary?: (text: string) => void
    ondone?: () => void
  } = $props()
  const card = useCard()
  const narrow = $derived(row && card.narrow)
  const target = (): PersonTarget => ({ photo, place })
  const title = $derived(place ? 'Имя на месте' : 'Имя на фото')

  const DELAY = 500
  // k — ключ Person, w — имя поля в who шаблона (у предмета они разные: subj / subject).
  const ALL = [
    { k: 'last', w: 'last', label: 'Фамилия', short: 'Фам.' },
    { k: 'first', w: 'first', label: 'Имя', short: 'Имя' },
    { k: 'middle', w: 'middle', label: 'Отчество', short: 'Отч.' },
    { k: 'subj', w: 'subject', label: 'Предмет', short: 'Предм.' },
  ] as const
  // Предмет в карточке — у учителя виньетки или если он уже задан; раз показанное поле не пропадает под курсором.
  let subjectShown = $state(false)
  const FIELDS = $derived(only ? ALL.filter((f) => only.includes(f.w)) : ALL.filter((f) => f.k !== 'subj' || subjectShown))

  let draft = $state({ last: '', first: '', middle: '', subj: '' })
  // Значение свёрнутой группы (имя из файла тоже в draft — оно и есть действующее).
  $effect(() => {
    onsummary?.(nameSum({ last: draft.last, first: draft.first, patr: draft.middle }))
  })
  let fromFile = $state(false)
  let seatNote = $state('') // подсказка, если у фото видна табличка места
  let dirty = $state(false)
  let session = ''
  let root: HTMLElement | undefined
  let nSession = 0
  let seq = 0
  // Пауза и очередь отправок; edit.idle() досылает и ждёт их (смена альбома, сохранение, отмена).
  const sender = new PersonSend()
  let selectFirst = false // inline: после первой загрузки имени выделить текст в поле с фокусом

  // Действующее имя от Go: при смене фото и после любой правки. Свой набор (dirty) ответ не перебивает.
  $effect(() => {
    const t = target()
    void store.view.rev
    const n = ++seq
    loadPerson(t).then(
      (pv) => {
        if (n !== seq || dirty) return
        const c = cardOf(t, pv)
        draft = c.draft
        if (pv.teacher || pv.subject) subjectShown = true
        fromFile = c.fromFile
        seatNote = c.note
        if (selectFirst) {
          selectFirst = false
          void tick().then(() => selectFocused())
        }
      },
      () => {},
    )
  })

  // Что уйдёт в Go: считается в момент отправки; null — набора нет.
  function prepare(t: PersonTarget) {
    return () => {
      if (!dirty) return null
      // Имя ещё из файла, набран только предмет: имена не шлём — Go оставит имя из файла (решение 174).
      const p = personOf(fromFile ? { subj: draft.subj } : draft)
      const s = session
      dirty = false // дальше свежий ответ Go (имя из файла после очистки) можно показывать
      return () => savePerson(t, p, s)
    }
  }

  // Карточка закрыта или фото сменилось (родитель пересоздаёт компонент по фото): набранное досылаем.
  // Летящий вызов остаётся в edit.idle() и после размонтирования.
  $effect(() => {
    void target() // смена фото/места — досылка, как при закрытии
    const off = edit.onIdle(() => void sender.flush())
    return () => {
      off()
      if (sender.waiting) void sender.flush()
    }
  })

  function focus() {
    if (inline) {
      if (!session) session = sessionId(Date.now(), ++nSession)
      return
    }
    if (!dirty) session = sessionId(Date.now(), ++nSession)
  }
  // Выделить весь текст в поле с фокусом: набор заменяет значение, а не дописывается к нему.
  function selectFocused() {
    const el = document.activeElement
    if (el instanceof HTMLInputElement && root?.contains(el)) el.select()
  }
  // Окошко на холсте открыто — курсор в первом поле. Значение приходит от Go позже (Api.Person), поэтому
  // выделение — после первого ответа; select() сейчас выделил бы пустое поле.
  onMount(() => {
    if (!inline) return
    root?.querySelector('input')?.focus()
    selectFirst = true
  })

  // Enter: следующее поле, на последнем — закрыть окошко.
  function key(e: KeyboardEvent) {
    if (!inline || e.key !== 'Enter') return
    e.preventDefault()
    const list = [...(root?.querySelectorAll('input') ?? [])]
    const next = list[list.indexOf(e.currentTarget as HTMLInputElement) + 1]
    if (next) {
      next.focus()
      next.select()
    } else ondone?.()
  }
  function input(k: (typeof ALL)[number]['k'], v: string) {
    selectFirst = false
    draft[k] = v
    dirty = true
    if (k !== 'subj') fromFile = false
    sender.schedule(prepare(target()), DELAY)
  }
</script>

{#snippet fld(f: (typeof ALL)[number])}
  <input
    type="text"
    class:file={fromFile && f.k !== 'subj'}
    maxlength="60"
    placeholder={narrow ? f.short : f.label}
    aria-label={f.label}
    title={row ? draft[f.k] || f.label : undefined}
    value={draft[f.k]}
    onfocus={focus}
    oninput={(e) => input(f.k, e.currentTarget.value)}
    onkeydown={key}
    onblur={() => void sender.flush(prepare(target()))}
  />
{/snippet}

<div
  bind:this={root}
  class="person"
  class:inline
  class:row
  class:bare={row && !cap}
  role="group"
  aria-label={title}
  {...inline || place ? {} : { 'data-guide': 'person' }}
>
  {#if !inline && cap}<div class="cap">{title}</div>{/if}
  {#if row}
    <div class="fio">
      {#each FIELDS.filter((f) => f.k !== 'subj') as f (f.k)}{@render fld(f)}{/each}
    </div>
    {#each FIELDS.filter((f) => f.k === 'subj') as f (f.k)}{@render fld(f)}{/each}
  {:else}
    {#each FIELDS as f (f.k)}{@render fld(f)}{/each}
  {/if}
  {#if fromFile}<div class="note">из имени файла</div>{/if}
  {#if seatNote}<div class="note">{seatNote}</div>{/if}
</div>

<style>
  .person {
    margin-top: 10px;
    padding-top: 8px;
    border-top: 1px solid var(--line);
    display: grid;
    gap: 4px;
  }
  /* Группа «Имя на фото» подписана заголовком: своих линии и отступа нет */
  .person.bare {
    margin: 0;
    padding: 0;
    border: 0;
  }
  /* Три поля в строку; поле в фокусе шире соседних (0.26-124-22), остальные сжимаются, но не до нуля */
  .fio {
    display: flex;
    gap: 4px;
  }
  .row .fio input {
    flex: 1 1 0;
    min-width: 0;
    padding: 0 4px;
  }
  .row .fio input:focus {
    flex-grow: 3;
  }
  .person.inline {
    margin: 0;
    padding: 0;
    border: 0;
    grid-template-columns: minmax(0, 1fr);
  }
  /* Окошко на холсте: поля в стиле тёмных полей интерфейса, а не белые браузерные, и строго внутри плашки */
  .inline input {
    min-width: 0;
    height: 28px;
    padding: 0 8px;
    font: inherit;
    font-size: 12px;
    color: var(--text);
    background: #3a3a3e;
    border: 1px solid var(--line);
    border-radius: 6px;
  }
  .inline input.file {
    color: var(--muted);
  }
  .inline input:focus {
    outline: none;
    border-color: var(--accent);
  }
  .cap {
    font-size: 12px;
    font-weight: 600;
  }
  input {
    width: 100%;
    height: 24px;
    padding: 0 6px;
    box-sizing: border-box;
    font-size: 12px;
  }
  .file {
    color: var(--muted);
  }
  .note {
    font-size: 11px;
    color: var(--muted);
  }
</style>
