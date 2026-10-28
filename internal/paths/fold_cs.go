//go:build !windows && !darwin

package paths

// foldCase — ext4 и прочие ФС Linux различают регистр: `/x/Весна` и `/x/ВЕСНА` — две папки.
const foldCase = false
