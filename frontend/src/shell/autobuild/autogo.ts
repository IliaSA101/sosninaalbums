// «Собрать» / «Пересобрать…» окна автосборки: запомнить выбор окна и отдать сборку в store. В режиме альбома (107-5)
// первая пересборка сперва спрашивает («Пересобрать альбом?»); пересборка цепочки (вариант +1) — без вопроса.
import { keymap } from '../../base/keys/keymap.svelte'
import { store } from '../../base/core/store.svelte'
import { modes } from '../../base/prefs/view.svelte'
import type { Unit } from '../../base/geom/sheet'
import { rebuildAskLines } from './autoalbum'
import type { AutoDialog } from './autobuild.svelte'
import { memOf } from './autobuild'
import { rebuildAsk } from './rebuildask.svelte'

export interface GoOpts {
  famOn: boolean // без функции «Варианты раскладки» фильтра в окне не было — память семейств не трогаем
  grayOn: boolean // так же ч/б без «Ч/б и тон фото»
  unit: Unit
}

// Запоминаем и собираем то, что подтвердил Go: N после ответа, а не то, что успели набрать.
function submit(u: AutoDialog, o: GoOpts): Promise<boolean> {
  const album = u.ph.album
  return u.submit((n, range, per, by, variety, families) => {
    // Режим альбома не пишет N и «среднее»: следующая обычная сборка не должна открыться с N альбома
    modes.rememberAuto(
      album ? modes.autoSpreads : n,
      memOf(range),
      album ? modes.autoPer : per,
      album ? modes.autoBy : by,
      variety,
      range.series,
      o.famOn ? families : modes.autoFamilies,
    )
    modes.rememberPhotos(u.ph.similar, o.grayOn ? u.ph.gray : modes.autoGray)
    // Вариант совпал с текущим, окно открыто (080-1): показываем следующий
    return store.autoBuild(n, range, variety, families, u.ph.opts, u.plan?.similar ?? 0, (v) => u.ph.setVariant(v))
  })
}

export async function goBuild(u: AutoDialog, o: GoOpts): Promise<void> {
  if (!u.ph.album || u.ph.again) {
    await submit(u, o)
    return
  }
  // Вопрос с числами плана: ждём ответа Go, чтобы не спрашивать про устаревшее
  u.settle()
  await u.idle()
  const a = u.plan?.album
  if (!u.canBuild || !a) return
  const lines = rebuildAskLines(a, u.n, u.plan?.similar ?? 0, !!store.view.cover, keymap.hint('undo'), o.unit)
  rebuildAsk.open(lines, () => void submit(u, o))
}
