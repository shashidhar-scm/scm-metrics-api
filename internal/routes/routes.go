package routes

import "net/http"

type Middleware func(http.HandlerFunc) http.HandlerFunc

type Handlers struct {
	Root              http.HandlerFunc
	Ingest            http.HandlerFunc
	Servers           http.HandlerFunc
	ServersStatus     http.HandlerFunc
	ServersStatusCity http.HandlerFunc
	MetricsLatest     http.HandlerFunc
	MetricsHistory    http.HandlerFunc
	SeriesList        http.HandlerFunc
	SeriesLatest      http.HandlerFunc
	SeriesQuery       http.HandlerFunc
}

func Register(mux *http.ServeMux, mw Middleware, handlers Handlers) {
	if mux == nil {
		mux = http.DefaultServeMux
	}

	add := func(path string, handler http.HandlerFunc) {
		if handler == nil {
			return
		}
		if mw != nil {
			handler = mw(handler)
		}
		mux.HandleFunc(path, handler)
	}

	add("/", handlers.Root)
	add("/api/metrics", handlers.Ingest)
	add("/api/servers", handlers.Servers)
	add("/api/servers/status", handlers.ServersStatus)
	add("/api/servers/status/city", handlers.ServersStatusCity)
	add("/api/metrics/latest", handlers.MetricsLatest)
	add("/api/metrics/history", handlers.MetricsHistory)
	add("/api/series", handlers.SeriesList)
	add("/api/series/latest", handlers.SeriesLatest)
	add("/api/series/query", handlers.SeriesQuery)
}
