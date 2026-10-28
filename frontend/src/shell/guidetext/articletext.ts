// Текст статьи для показа и поиска: метки клавиш ({key}), затем куски по уровню ({can}) — одно место для
// GuideCard, поиска и подсказки miss (080-9).
import { keymap } from '../../base/keys/keymap.svelte'
import { prefs } from '../../base/prefs/prefs.svelte'
import type { FeatureId } from '../../base/prefs/levels'
import { fillLevels } from './leveltext'

export const articleText = (s: string, can: (id: FeatureId) => boolean = prefs.can): string => fillLevels(keymap.text(s), can)
