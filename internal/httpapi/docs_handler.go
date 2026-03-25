package httpapi

import (
	"encoding/base64"
	"fmt"
	"net/http"

	"kintsugi-storage/internal/apidocs"
)

var specBase64 = base64.StdEncoding.EncodeToString(apidocs.OpenAPI)

func DocsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprintf(w, docsPage, specBase64)
}

const docsPage = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>Kintsugi Storage API Docs</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist/swagger-ui.css" />
  <style>
    body { margin: 0; background: #fafafa; }
  </style>
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist/swagger-ui-bundle.js"></script>
  <script>
    window.onload = function () {
      var raw = atob('%s');
      var blob = new Blob([raw], { type: 'application/yaml' });
      var url = URL.createObjectURL(blob);
      SwaggerUIBundle({
        url: url,
        dom_id: '#swagger-ui'
      });
    };
  </script>
</body>
</html>`
