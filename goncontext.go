package main

import (
	//"context"
	//"os"
	//"regexp"
	//"sync"
	"bytes"
	"crypto/sha1"
	"database/sql"
	"encoding/base64"
	"fmt"
	"golang.org/x/crypto/pbkdf2"
	"html/template"
	"log"
	"net/http"
	"path/filepath"
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

type UserSession struct {
	Uid      int64  // UID for user who signed in
	Username string // Username for session
	Avatar   string
}

type GonContext struct {
	config    *Config
	decoder   *schema.Decoder
	templates *template.Template
	sessions  map[string]*UserSession
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
		sessions:  make(map[string]*UserSession),
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
	result["title"] = "Gontentapi"
	cookie, err := r.Cookie(gctx.config.LoginCookie)
	if err != nil {
		if err != http.ErrNoCookie {
			log.Printf("Cookie error: %s", err)
		}
	} else {
		user, ok := gctx.sessions[cookie.Value]
		if ok {
			result["user"] = user
			result["loggedin"] = true
		}
	}
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

// Create hash in the same way old contentapi did it. useful for login
func GetHash(password []byte, salt []byte) []byte {
	const (
		SaltBits       = 256
		HashBits       = 512
		HashIterations = 10000
	)
	return pbkdf2.Key(password, salt, HashIterations, HashBits/8, sha1.New)
}

func (gctx *GonContext) TestLogin(username string, password string) (*UserSession, error) {
	var result UserSession
	var passhashb64, saltb64 string
	// Lookup user in database
	err := gctx.contentdb.QueryRow("SELECT id,username,avatar,password,salt FROM users WHERE username = ?", username).Scan(
		&result.Uid, &result.Username, &result.Avatar, &passhashb64, &saltb64)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("WARN: attempted login for non-existent user %s", username)
			return nil, &utils.BadRequest{Message: "User not found!"}
		} else {
			return nil, err
		}
	}
	// Compare hash
	salt, err := base64.StdEncoding.DecodeString(saltb64)
	if err != nil {
		return nil, err
	}
	testhash := GetHash([]byte(password), salt)
	passhash, err := base64.StdEncoding.DecodeString(passhashb64)
	if err != nil {
		return nil, err
	}
	if bytes.Equal(testhash, passhash) {
		return &result, nil
	} else {
		log.Printf("WARN: password failure for user %s [%d]", username, result.Uid)
		return nil, &utils.BadRequest{Message: "Password failure!"}
	}
}
