package albums

import _ "embed" // источник OpenAPI встроен в пакет

// OpenAPI — контракт /v1 (v1/openapi.yaml): документ для чужих реализаций и тест сверки.
//
//go:embed v1/openapi.yaml
var OpenAPI []byte
