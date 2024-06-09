package main

import (
	//"context"
	"fmt"
	"html/template"
	"log"
	"net/http"
	//"os"
	"path/filepath"
	//"regexp"
	//"sync"
	"time"

	"github.com/gorilla/schema"
	"github.com/jmoiron/sqlx"

	"github.com/randomouscrap98/gontentapi/utils"

	_ "github.com/mattn/go-sqlite3"
)

const (
	BusyTimeout = 5000
	Version     = "0.1"
)

type GonContext struct {
	config    *Config
	decoder   *schema.Decoder
	templates *template.Template
	//chatlogIncludeRegex *regexp.Regexp
	created time.Time
	//drawDataMu          sync.Mutex
	contentdb *sqlx.DB
}

func NewContext(config *Config) (*GonContext, error) {
	// chatlogIncludeRegex, err := regexp.Compile(config.ChatlogIncludeRegex)
	// if err != nil {
	// 	return nil, err
	// }
	// We initialize the templates first because we don't really need
	// hot reloading (also it's just better for performance... though memory usage...
	templates, err := template.New("alltemplates").Funcs(template.FuncMap{
		"RawHtml": func(c string) template.HTML { return template.HTML(c) },
		"RawUrl":  func(c string) template.URL { return template.URL(c) },
	}).ParseGlob(filepath.Join(config.Templates, "*.tmpl"))

	if err != nil {
		return nil, err
	}

	contentdb, err := sqlx.Open("sqlite3", fmt.Sprintf("%s?_busy_timeout=%d", config.Database, BusyTimeout))
	if err != nil {
		return nil, err
	}

	decoder := schema.NewDecoder()
	decoder.IgnoreUnknownKeys(true)

	// Now we're good to go
	return &GonContext{
		config:    config,
		templates: templates,
		decoder:   decoder,
		created:   time.Now(),
		contentdb: contentdb,
	}, nil
}

// Retrieve the default data for any page load. Add your additional data to this
// map before rendering
func (gctx *GonContext) GetDefaultData(r *http.Request) map[string]any {
	rinfo := utils.GetRuntimeInfo()
	result := make(map[string]any)
	result["root"] = template.URL(gctx.config.RootPath)
	result["appversion"] = Version
	result["runtimeInfo"] = rinfo
	result["requestUri"] = r.URL.RequestURI()
	result["cachebust"] = gctx.created.Format(time.RFC3339)
	//"RawHtml": func(c string) template.HTML { return template.HTML(c) },
	return result
}

// Call this instead of directly accessing templates to do a final render of a page
func (gctx *GonContext) RunTemplate(name string, w http.ResponseWriter, data any) {
	err := gctx.templates.ExecuteTemplate(w, name, data)
	if err != nil {
		log.Printf("ERROR: can't load template: %s", err)
		http.Error(w, "Template load error (internal server error!)", http.StatusInternalServerError)
	}
}
