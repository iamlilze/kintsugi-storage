package apidocs

import _ "embed"

// OpenAPI содержит встроенную OpenAPI-спецификацию сервиса.
//
//go:embed openapi.yaml
var OpenAPI []byte
