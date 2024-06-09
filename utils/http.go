package utils

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

const (
	DefaultCacheControl = "public, max-age=15552000" // 6 months
)

// Adds a robots.txt that disallows everything to the router. It of course
// is served at root. It might be better to include a robots.txt in the
// static file list to give more control, however...
func AngryRobots(r *chi.Mux) {
	r.Get("/robots.txt", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("User-agent: *\nDisallow: /\n"))
	})
}

// Don't allow directory listings witin the specified directory.
// Taken from https://www.alexedwards.net/blog/disable-http-fileserver-directory-listings
func NoDirectoryListing(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/") {
			http.NotFound(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// Taken from: https://github.com/go-chi/chi/blob/master/_examples/fileserver/main.go
// FileServer conveniently sets up a http.FileServer handler to serve
// static files from a http.FileSystem.
func FileServerRaw(r chi.Router, path string, root http.FileSystem, listdir bool) {
	if strings.ContainsAny(path, "{}*") {
		panic("FileServer does not permit any URL parameters.")
	}

	// There's a bug here: what if path is empty?
	if path != "/" && path[len(path)-1] != '/' {
		r.Get(path, http.RedirectHandler(path+"/", 301).ServeHTTP)
		path += "/"
	}
	path += "*"

	var handler = http.FileServer(root)
	if !listdir {
		handler = NoDirectoryListing(handler)
	}

	r.Get(path, func(w http.ResponseWriter, r *http.Request) {
		rctx := chi.RouteContext(r.Context())
		pathPrefix := strings.TrimSuffix(rctx.RoutePattern(), "/*")
		fs := http.StripPrefix(pathPrefix, handler)
		w.Header().Set("Cache-Control", DefaultCacheControl)
		fs.ServeHTTP(w, r)
	})
}

func FileServer(r chi.Router, path string, local string, listdir bool) error {
	staticPath, err := filepath.Abs(local)
	if err != nil {
		panic(err)
	}
	FileServerRaw(r, path, http.Dir(staticPath), listdir)
	return nil
}

func RespondPlaintext(data []byte, w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, err := w.Write(data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func RespondJson(v any, w http.ResponseWriter, extra func(*json.Encoder)) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	encoder := json.NewEncoder(w)
	if extra != nil {
		extra(encoder)
	}
	err := encoder.Encode(v)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func DeleteCookie(name string, w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:    name,
		Value:   "",
		Expires: time.Now().Add(-time.Hour),
	})
}

// Return the value from an integer cookie
func GetCookieOrDefault[T any](name string, r *http.Request, def T, parse func(string) (T, error)) T {
	cookie, err := r.Cookie(name)
	if err != nil {
		return def
	}
	parsed, err := parse(cookie.Value)
	if err != nil {
		return def
	}
	return parsed
}
