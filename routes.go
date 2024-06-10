package main

import (
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
)

func SetupRoutes(r *chi.Mux, gctx *GonContext) error {
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		data := gctx.GetDefaultData(r)
		gctx.RunTemplate("index.tmpl", w, data)
	})
	r.Post("/login", func(w http.ResponseWriter, r *http.Request) {
		username := r.FormValue("username")
		password := r.FormValue("password")
		log.Printf("Username: %s   Password: %s", username, password)

		returnUrl := r.FormValue("return")
		http.Redirect(w, r, returnUrl, http.StatusSeeOther)
	})
	return nil
}
