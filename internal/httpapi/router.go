package httpapi

import "net/http"

type RouterDependencies struct {
	Objects *ObjectsHandler
	Probes  *ProbesHandler
	Metrics http.Handler
}

func NewRouter(deps RouterDependencies) http.Handler {
	mux := http.NewServeMux()

	if deps.Objects != nil {
		mux.HandleFunc("/objects/", func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodPut:
				deps.Objects.PutObject(w, r)
			case http.MethodGet:
				deps.Objects.GetObject(w, r)
			default:
				writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			}
		})
	}

	if deps.Probes != nil {
		mux.HandleFunc("/probes/liveness", deps.Probes.Liveness)
		mux.HandleFunc("/probes/readiness", deps.Probes.Readiness)
	}

	if deps.Metrics != nil {
		mux.Handle("/metrics", deps.Metrics)
	}

	mux.HandleFunc("/docs", DocsHandler)

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "route not found")
	})

	return mux
}
