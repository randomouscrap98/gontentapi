package main

import (
	//"context"
	//"regexp"
	"bytes"
	"crypto/sha1"
	"database/sql"
	"encoding/base64"
	"fmt"
	"golang.org/x/crypto/pbkdf2"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/schema"
	"github.com/jmoiron/sqlx"

	"github.com/randomouscrap98/gontentapi/contentapi"
	"github.com/randomouscrap98/gontentapi/utils"

	_ "github.com/mattn/go-sqlite3"
)

const (
	BusyTimeout = 5000
	Version     = "0.3.0"
)

type UserSession struct {
	Uid      int64  // UID for user who signed in
	Username string // Username for session
	Avatar   string
	Created  time.Time // When session was created
}

type GonContext struct {
	config        *Config
	decoder       *schema.Decoder
	templates     *template.Template
	sessions      map[string]*UserSession
	sessionLock   sync.Mutex
	thumbnailLock sync.Mutex
	created       time.Time
	contentdb     *sqlx.DB
	//chatlogIncludeRegex *regexp.Regexp
}

func NewContext(config *Config) (*GonContext, error) {
	// chatlogIncludeRegex, err := regexp.Compile(config.ChatlogIncludeRegex)
	// if err != nil {
	// 	return nil, err
	// }
	// Make sure we can initialize the thumbnail folder
	err := os.MkdirAll(config.ThumbnailFolder, 0750)
	if err != nil {
		return nil, err
	}

	// We initialize the templates first because we don't really need
	// hot reloading (also it's just better for performance... though memory usage...
	templates, err := template.New("alltemplates").Funcs(template.FuncMap{
		"RawHtml":      func(c string) template.HTML { return template.HTML(c) },
		"RawUrl":       func(c string) template.URL { return template.URL(c) },
		"UploadUrl":    func(c string) string { return fmt.Sprintf("%s/uploads/%s", config.RootPath, c) },
		"ThumbnailUrl": func(c string) string { return fmt.Sprintf("%s/thumbnails/%s", config.RootPath, c) },
		"PageUrl": func(c *contentapi.Content) string {
			url := config.RootPath + "/pages"
			if c.Id != 0 { // The root page (or otherwise). DON'T check hash: we WANT it to fail if hash empty
				url += "/" + c.Hash
			}
			return url
		},
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

func (gctx *GonContext) IsExpired(user *UserSession) bool {
	return time.Now().After(user.Created.Add(time.Duration(gctx.config.LoginExpire)))
}

// Return the current user session if it exists, otherwise return nil. There are
// "no errors" because, if the cookie doesn't exist, it's the same as if the
// cookie is expired. the only time something is invalid is if something went
// wrong RETRIEVING the cookie, which is very unlikely (and we just log it)
func (gctx *GonContext) GetCurrentUser(r *http.Request) *UserSession {
	cookie, err := r.Cookie(gctx.config.LoginCookie)
	if err != nil {
		if err != http.ErrNoCookie {
			log.Printf("Cookie error: %s", err)
		}
	} else {
		gctx.sessionLock.Lock()
		defer gctx.sessionLock.Unlock() // SHOULD be fine... don't do too much here anyway!
		user, ok := gctx.sessions[cookie.Value]
		if ok && !gctx.IsExpired(user) {
			return user
		}
	}
	return nil
}

// Retrieve the default data for any page load. Add your additional data to this
// map before rendering
func (gctx *GonContext) GetDefaultData(r *http.Request, user *UserSession) map[string]any {
	rinfo := utils.GetRuntimeInfo()
	result := make(map[string]any)
	result["root"] = template.URL(gctx.config.RootPath)
	result["appversion"] = Version
	result["runtimeInfo"] = rinfo
	result["requestUri"] = gctx.config.RootPath + r.URL.RequestURI()
	result["cachebust"] = gctx.created.Format(time.RFC3339)
	result["title"] = "Gontentapi"
	if user != nil {
		result["user"] = user
		result["loggedin"] = true
	}
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
		result.Created = time.Now()
		return &result, nil
	} else {
		log.Printf("WARN: password failure for user %s [%d]", username, result.Uid)
		return nil, &utils.BadRequest{Message: "Password failure!"}
	}
}

// Attempt to add a new session, returning the generated sessionID. Threadsafe,
// and removes old sessions when done
func (gctx *GonContext) AddSession(user *UserSession) (string, error) {
	// It's a new user, put them in the session
	sessid_raw, err := uuid.NewRandom()
	if err != nil { //handleError(err, w) {
		return "", err
	}
	sessid := sessid_raw.String()
	gctx.sessionLock.Lock()
	defer gctx.sessionLock.Unlock()
	// Remove old sessions (expired sessions)
	removed := 0
	for k, v := range gctx.sessions {
		if gctx.IsExpired(v) {
			delete(gctx.sessions, k)
			removed += 1
		}
	}
	if removed > 0 {
		log.Printf("Removed %d old sessions", removed)
	}
	// If sessions is still too large, just reject it
	if len(gctx.sessions) >= gctx.config.MaxSessions {
		return "", fmt.Errorf("Too many sessions: %d", gctx.config.MaxSessions) // This is an unexpected error
	}
	gctx.sessions[sessid] = user
	return sessid, nil
}

func MakeRoot(c *contentapi.Content) *contentapi.Content {
	if c == nil {
		c = &contentapi.Content{}
	}
	c.Name = "Root"
	return c
}

// Retrieve users for the given uids
func (gctx *GonContext) GetUsers(uids ...int64) ([]contentapi.User, error) {
	// To reduce strain on the system (and because the params must be "any")
	// we only put params in that haven't been seen
	params := utils.UniqueParams(uids...)
	q := contentapi.NewQuery()
	q.Sql = "SELECT " + contentapi.GetUserFields("") + " FROM users WHERE id IN ("
	q.AddQueryParams(params...)
	q.Sql += ")"
	q.Finalize()

	users := make([]contentapi.User, 0)
	err := gctx.contentdb.Select(&users, q.Sql, q.Params...)

	if err != nil {
		return nil, err
	}

	return users, nil
}
