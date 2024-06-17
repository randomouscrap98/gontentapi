package main

import (
	//"context"
	//"os"
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
	Version     = "0.1"
)

type UserSession struct {
	Uid      int64  // UID for user who signed in
	Username string // Username for session
	Avatar   string
	Created  time.Time // When session was created
}

type GonContext struct {
	config      *Config
	decoder     *schema.Decoder
	templates   *template.Template
	sessions    map[string]*UserSession
	sessionLock sync.Mutex
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
		"RawHtml":   func(c string) template.HTML { return template.HTML(c) },
		"RawUrl":    func(c string) template.URL { return template.URL(c) },
		"UploadUrl": func(c string) string { return fmt.Sprintf("%s/uploads/%s", config.RootPath, c) },
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
	result["requestUri"] = r.URL.RequestURI()
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

// Add all the page data (main page, subpages, etc) for
func (gctx *GonContext) AddPageData(hash string, user *UserSession, data map[string]any) error {
	var uid int64
	if user != nil {
		uid = int64(user.Uid)
	}

	var mainpage contentapi.Content

	if hash == "" {
		// This is the root page
		mainpage.Name = "Root"
	} else {
		q := contentapi.NewQuery()
		q.Sql = "SELECT id,name,hash,text,parentId,contentType,createDate FROM content WHERE hash = ?"
		q.AddParams(hash)
		q.AndViewable("id", uid)
		q.Finalize()
		err := gctx.contentdb.Get(&mainpage, q.Sql, q.Params...)

		if err != nil {
			if err == sql.ErrNoRows {
				return &utils.NotFound{Message: fmt.Sprintf("No content with hash %s", hash)}
			} else {
				return err
			}
		}
	}

	data["title"] = mainpage.Name
	data["mainpage"] = mainpage

	q := contentapi.NewQuery()
	q.Sql = "SELECT id,name,hash,'' as text,parentId,contentType,createDate FROM content " +
		"WHERE parentId = ? AND contentType <> ?"
	q.AddParams(mainpage.Id, contentapi.ContentType_File)
	q.AndViewable("id", uid)
	q.Order = "name"
	q.Finalize()

	subpages := make([]contentapi.Content, 0)
	err := gctx.contentdb.Select(&subpages, q.Sql, q.Params...)

	if err != nil {
		return err
	}

	data["subpages"] = subpages

	// result := make([]QueryByPuzzleset, 0)
	// err := mctx.sudokuDb.Select(&result,
	// 	"SELECT p.pid, (c.cid IS NOT NULL) as completed, (i.ipid IS NOT NULL) as paused, c.completed as completedOn, i.paused as pausedOn "+
	// 		"FROM puzzles p LEFT JOIN "+
	// 		"completions c ON c.pid=p.pid LEFT JOIN "+
	// 		"inprogress i ON i.pid=p.pid "+
	// 		"WHERE puzzleset=? AND (c.uid=? OR c.uid IS NULL) AND "+
	// 		"(i.uid=? OR i.uid IS NULL) "+
	// 		"GROUP BY p.pid ORDER BY p.pid ",
	// 	puzzleset, uid, uid,
	// )
	// var result QueryByPid
	// err := mctx.sudokuDb.Get(&result,
	// 	"SELECT p.*, COALESCE(i.puzzle,'') as playersolution, COALESCE(i.seconds,0) as seconds FROM puzzles p LEFT JOIN "+
	// 		"inprogress i ON p.pid=i.pid WHERE p.pid=? AND "+
	// 		"(i.uid=? OR i.uid IS NULL)",
	// 	pid, uid,
	// )
	return nil
}
