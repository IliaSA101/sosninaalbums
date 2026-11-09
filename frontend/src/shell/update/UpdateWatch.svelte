<script lang="ts">
  import { EventsOn } from '../../../wailsjs/runtime/runtime'
  import { run } from '../menu/commands'
  import { updateUi } from './update.svelte'
  import type { UpdateInfo, UpdateProgress } from './updateapi'

  // Смонтирован всегда (как PacksDialog): тосты и ход нужны и при закрытом «О программе».
  // События Go подписываются здесь; итог прошлого обновления (Just/Failed) спрашивается один раз при запуске.
  $effect(() => {
    updateUi.host = { whatsNew: () => run('whatsNew') }
    const off = [
      EventsOn('update:offer', (i: UpdateInfo) => updateUi.offered(i)),
      EventsOn('update:state', (i: UpdateInfo) => updateUi.changed(i)),
      EventsOn('update:progress', (p: UpdateProgress) => updateUi.progressed(p)),
    ]
    void updateUi.start()
    return () => off.forEach((f) => f?.())
  })
</script>
