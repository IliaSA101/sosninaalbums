<script lang="ts">
  import { prefs } from '../../base/prefs/prefs.svelte'
  import type { SpreadView } from '../../base/core/types'
  import { isCover } from '../../base/geom/sheet'
  import { makeVignette } from '../vignette/vigact'
  import { vigDialog } from '../vignette/vigsession.svelte'
  import { noSeatsAction, noSeatsLine } from './plantext'

  // Пустой предпросмотр (071-11): на листе нет ни мест, ни фото. Кнопка — по состоянию листа (noSeatsAction):
  // обычный лист сначала становится виньеткой (как «Сделать виньеткой» в ленте), окно открывается на уже виньетке.
  let { sp, onCap }: { sp: SpreadView; onCap: string } = $props()
  const action = $derived(noSeatsAction(sp, prefs.can))
  let busy = $state(false)

  async function setup() {
    if (action === 'setup') return vigDialog.open(sp.id)
    if (busy) return
    busy = true
    try {
      if (await makeVignette(sp)) vigDialog.open(sp.id)
    } finally {
      busy = false
    }
  }
</script>

<li class="none empty">
  {noSeatsLine(onCap, isCover(sp))}
  {#if action !== 'none'}<button class="ghost" disabled={busy} onclick={setup}>Настроить виньетку…</button>{/if}
</li>

<style>
  /* пустое состояние — обычный абзац: в сетке строки фраза рвалась по слову в первой узкой колонке */
  .empty {
    display: block;
    line-height: 1.4;
    color: var(--muted);
  }
  .empty button {
    height: 24px;
    margin-left: 8px;
    font-size: 12px;
    padding: 0 10px;
  }
</style>
