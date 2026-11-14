//go:build unix

package imgfile

import (
	"os"
	"syscall"
)

// openFlag — на unix открытие канала (FIFO) без O_NONBLOCK ждёт писателя вечно, раньше проверки
// «обычный файл» (аудит 0.23-sec). Для обычного файла флаг ни на что не влияет.
const openFlag = os.O_RDONLY | syscall.O_NONBLOCK
