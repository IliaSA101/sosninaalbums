//go:build !unix

package imgfile

import "os"

// openFlag — вне unix (Windows) каналы по пути не открываются, обычное чтение.
const openFlag = os.O_RDONLY
