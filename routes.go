package main

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func SetupRoutes(r *chi.Mux, gctx *GonContext) error {
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		data := gctx.GetDefaultData(r)
		gctx.RunTemplate("index.tmpl", w, data)
	})
	return nil
}
