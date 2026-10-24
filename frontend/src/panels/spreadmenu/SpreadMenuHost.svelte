<script lang="ts">
  import { store } from '../../base/core/store.svelte'
  import { sheetIn, unitOf } from '../../base/geom/sheet'
  import { anyVaries } from '../gallery/variants'
  import SpreadMenu from './SpreadMenu.svelte'
  import type { SpreadMenuCtl } from './menuact.svelte'

  // Меню группы по состоянию ctl: лента и «Все развороты» вставляют его одинаково (103 §1) — пункты, порядок и вопросы общие.
  let { ctl }: { ctl: SpreadMenuCtl } = $props()
</script>

{#if ctl.menu}
  <SpreadMenu
    x={ctl.menu.x}
    y={ctl.menu.y}
    count={ctl.menu.ids.length}
    kind={sheetIn(store.view, ctl.menu.ids[0])?.kind}
    unit={unitOf(store.view.settings)}
    onVignette={(act) => ctl.vignette(act)}
    onVary={() => ctl.vary()}
    onRename={() => ctl.rename()}
    varies={anyVaries(store.view.spreads, ctl.menu.ids)}
    cut={ctl.cut}
    onCut={() => ctl.doCut()}
    join={ctl.join}
    onJoin={() => ctl.doJoin()}
    onInsert={(where) => ctl.insert(where)}
    onDelete={() => ctl.del()}
    onclose={ctl.close}
  />
{/if}
