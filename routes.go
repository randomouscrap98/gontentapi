package main

import (
	"fmt"
	"image"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	_ "image/gif"
	"image/jpeg"
	_ "image/jpeg"
	_ "image/png"

	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"

	"github.com/disintegration/imaging"
	"github.com/go-chi/chi/v5"

	"github.com/randomouscrap98/gontentapi/utils"
)

func handleError(err error, w http.ResponseWriter) bool {
	if err != nil {
		log.Printf("REQUEST ERROR: %s", err)
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
		http.Redirect(w, r, gctx.config.RootPath+"/pages", http.StatusFound)
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
	r.Get("/comments/{slug}", func(w http.ResponseWriter, r *http.Request) {
		if handleError(r.ParseForm(), w) {
			return
		}
		user := gctx.GetCurrentUser(r)
		data := gctx.GetDefaultData(r, user)
		var search CommentSearch
		if handleError(gctx.decoder.Decode(&search, r.Form), w) {
			return
		}
		_, iframe := r.Form["iframe"] // Iframe is if it exists at all, not the value
		comments, err := gctx.AddCommentData(chi.URLParam(r, "slug"), &search, user, data)
		if handleError(err, w) {
			return
		}
		if iframe {
			// Special iframe system, separate from search (but adjacent/uses same fields)
			params := url.Values{}
			params.Add("iframe", "1")
			if search.Page > 0 {
				params.Set("page", fmt.Sprint(search.Page-1))
				data["newerpageurl"] = "?" + params.Encode()
			}
			if len(comments) == gctx.config.CommentsPerPage {
				params.Set("page", fmt.Sprint(search.Page+1))
				data["olderpageurl"] = "?" + params.Encode()
			}
			data["iframe"] = true
		}
		gctx.RunTemplate("comments.tmpl", w, data)
	})
	r.Get("/search", func(w http.ResponseWriter, r *http.Request) {
		if handleError(r.ParseForm(), w) {
			return
		}
		user := gctx.GetCurrentUser(r)
		data := gctx.GetDefaultData(r, user)
		var search Search
		if handleError(gctx.decoder.Decode(&search, r.Form), w) {
			return
		}
		// We now have a search. Add the search data and return the rendered page
		if handleError(gctx.AddSearchResults(&search, user, data), w) {
			return
		}
		gctx.RunTemplate("search.tmpl", w, data)
	})
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
	r.Get("/thumbnails/{slug:[a-z0-9_-]+}", func(w http.ResponseWriter, r *http.Request) {
		imgslug := chi.URLParam(r, "slug")
		thumbpath := filepath.Join(gctx.config.ThumbnailFolder, imgslug)
		// Must check for thumbnail in lock. Hopefully the thumbnail exists and we
		// skip all that generation crap
		gctx.thumbnailLock.Lock()
		defer gctx.thumbnailLock.Unlock()
		file, err := os.Open(thumbpath)
		if err != nil {
			// This is a weird error; we only handle non-existent thumbnails
			if !os.IsNotExist(err) {
				handleError(err, w)
				return
			}
			// Load the original image so we can make a thumbnail
			origfile, err := os.Open(filepath.Join(gctx.config.Uploads, imgslug))
			if handleError(err, w) {
				return
			}
			defer origfile.Close()
			img, _, err := image.Decode(origfile)
			// Then we just use a third party library to generate a thumbnail
			thumbimg := imaging.Thumbnail(img, gctx.config.ThumbnailSize, gctx.config.ThumbnailSize, imaging.Lanczos)
			outfile, err := os.Create(thumbpath)
			if handleError(err, w) {
				return
			}
			options := jpeg.Options{
				Quality: gctx.config.ThumbnailJpegQuality,
			}
			err = jpeg.Encode(outfile, thumbimg, &options)
			outfile.Close()
			if handleError(err, w) {
				return
			}
			file, err = os.Open(thumbpath)
			if handleError(err, w) {
				return
			}
		}
		// Serve the thumbnail. We don't go check the modtime, just use the
		// system start date (it's fine, the files shouldn't change during runtime
		// but MIGHT change between runs...)
		http.ServeContent(w, r, imgslug+".jpg", gctx.created, file)
		file.Close()
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
