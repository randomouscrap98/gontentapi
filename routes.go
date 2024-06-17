package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/randomouscrap98/gontentapi/utils"
)

func handleError(err error, w http.ResponseWriter) bool {
	if err != nil {
		switch e := err.(type) {
		case *utils.NotFound:
			http.Error(w, e.Error(), http.StatusNotFound)
		case *utils.BadRequest:
			http.Error(w, e.Error(), http.StatusBadRequest)
		default:
			http.Error(w, fmt.Sprintf("UNEXPECTED ERROR: %s", e), http.StatusInternalServerError)
		}
		return true
	}
	return false
}

func SetupRoutes(r *chi.Mux, gctx *GonContext) error {
	// --- Normal routes ---
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		// Index has nothing for now, just take them to the pages
		http.Redirect(w, r, "/pages", http.StatusFound)
	})
	pagesRoute := func(w http.ResponseWriter, r *http.Request) {
		user := gctx.GetCurrentUser(r)
		data := gctx.GetDefaultData(r, user)
		err := gctx.AddPageData(chi.URLParam(r, "slug"), user, data)
		if handleError(err, w) {
			return
		}
		gctx.RunTemplate("index.tmpl", w, data)
	}
	// Retrieving a page is the same whether you have a slug or not
	r.Get("/pages", pagesRoute)
	r.Get("/pages/{slug}", pagesRoute)
	r.Post("/login", func(w http.ResponseWriter, r *http.Request) {
		username := r.FormValue("username")
		password := r.FormValue("password")
		returnUrl := r.FormValue("return")
		// Lookup user in database
		user, err := gctx.TestLogin(username, password)
		if handleError(err, w) {
			return
		}
		// It's a new user, put them in the session
		sessid, err := gctx.AddSession(user)
		if handleError(err, w) {
			return
		}
		// Set the cookie
		http.SetCookie(w, &http.Cookie{
			Name:   gctx.config.LoginCookie,
			Value:  sessid,
			MaxAge: int(time.Duration(gctx.config.LoginExpire).Seconds()),
		})
		http.Redirect(w, r, returnUrl, http.StatusSeeOther)
	})
	r.Post("/logout", func(w http.ResponseWriter, r *http.Request) {
		returnUrl := r.FormValue("return")
		utils.DeleteCookie(gctx.config.LoginCookie, w)
		http.Redirect(w, r, returnUrl, http.StatusSeeOther)
	})
	// --- Static files ---
	utils.AngryRobots(r)
	err := utils.FileServer(r, "/static", gctx.config.StaticFiles, true)
	if err != nil {
		return err
	}
	log.Printf("Hosting static files at %s\n", gctx.config.StaticFiles)
	err = utils.FileServer(r, "/uploads", gctx.config.Uploads, false)
	if err != nil {
		return err
	}
	log.Printf("Hosting uploads at %s\n", gctx.config.StaticFiles)
	return nil
}
