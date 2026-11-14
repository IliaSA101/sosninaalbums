<script lang="ts">
  import { Api, store } from '../../base/core/store.svelte'
  import type { BackdropView } from '../../base/core/types'
  import { fadePct, pctFade } from './fade'
  import { fadeLive } from './fade.svelte'
  import { bdSession } from './bgmode'

  // Строка «Прозрачность» фото фона (спека 017): живой просмотр на oninput, один вызов Go на change.
  // Коммиты идут в очередь. Стрелки в одном фокусе сливаются в один шаг отмены через merge-сессию.
  let { b, spread }: { b: BackdropView; spread: string } = $props()

  let session = ''
  const pct = $derived(fadePct(fadeLive.of(spread, b.scope, b.fade)))
  const val = (e: Event) => +(e.currentTarget as HTMLInputElement).value

  function commit(e: Event) {
    const f = pctFade(val(e))
    const merge = session
    const area = b.scope
    return fadeLive.commit(() => store.run(Api.SetBackdropFade(spread, area, f, merge)))
  }

  // Chromium не шлёт change, если значение при отпускании равно исходному (30 → 50 → 30):
  // тогда просмотр залип бы и после Ctrl+Z холст врал.
  function blur() {
    session = ''
    if (pct === fadePct(b.fade)) fadeLive.release()
  }

  // Уход с разворота или из режима фона: просмотр не должен остаться на чужом развороте.
  $effect(() => {
    void spread
    void b.scope
    return () => {
      session = ''
      fadeLive.release()
    }
  })
</script>

<div class="fade" title="Чем больше, тем бледнее фото: сквозь него проступает цвет страницы">
  <span class="cap">Прозрачность</span>
  <input
    type="range"
    min="0"
    max="100"
    step="1"
    value={pct}
    aria-label="Прозрачность фото фона, %"
    aria-valuetext="{pct} %"
    onfocus={() => (session = bdSession())}
    onblur={blur}
    oninput={(e) => fadeLive.set(spread, b.scope, pctFade(val(e)))}
    onchange={commit}
    onpointerup={(e) => e.currentTarget.blur()}
  />
  <span class="num">{pct} %</span>
</div>

<style>
  .fade {
    display: flex;
    align-items: center;
    gap: 6px;
    margin-top: 6px;
    font-size: 12px;
  }
  .cap {
    flex: none;
    color: #cfcfd4;
  }
  input {
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
</style>
