package agentplaces

// DevOf на Windows: корень тома — это «X:\» (правило 149 его уже отсекает), точек монтирования
// внутри папок нет — устройство у всех путей «одно».
func DevOf(string) (dev uint64, ok bool) { return 0, true }
