// «Настройки альбома» правятся прямо сейчас (149-16, design §2.5): у формы SettingsForm есть непринятые правки (черновик
// отличается от того, что принято, — применяет «Применить») или фокус внутри раздела. Одно раскрытие раздела правкой не считается:
// флаг раскрытия лежит в localStorage и оставался бы «занят» навсегда. Читает shell/agents/agentbusy.ts.
export const settingsDraft = $state({ dirty: false, focus: false })

// Тело раздела: id собирает Section.svelte (`section-{name}`).
export const SETTINGS_BODY = '#section-settings'

export const settingsBusy = (): boolean => settingsDraft.dirty || settingsDraft.focus

// Фокус внутри раздела: relatedTarget при уходе фокуса — куда он пошёл (null — вне окна).
export const inSettings = (t: EventTarget | null): boolean => t instanceof Element && !!t.closest(SETTINGS_BODY)
