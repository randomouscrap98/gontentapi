package main

import (
	"fmt"
	"time"

	"github.com/randomouscrap98/gontentapi/utils"
)

type Config struct {
	Address      string         // Full address to host on (includes IP to limit to localhost/etc)
	ShutdownTime utils.Duration // Time to wait for server to shutdown
	StaticFiles  string         // Where the static files for ALL endpoints go
	HeaderLimit  int            // Maximum allowed header size
	Timeout      utils.Duration // How long a connection is allowed to last
	Database     string         // Path to the contentapi database file
	Uploads      string         // Path to all the uploaded files
	Templates    string         // Path to all the templates
	RootPath     string         // The root path to our service (the url path)
	LoginCookie  string         // Name of the login cookie
	LoginExpire  utils.Duration // How long the login cookie lasts
	MaxSessions  int            // How many total sessions can exist
	//SessionTrim  float32        // How much to trim the sessions by when it gets too full
}

func GetDefaultConfig_Toml() string {
	baseConfig := `# Config auto-generated on %s
Address=":5030"                # Where to run the server
ShutdownTime="10s"             # How long to wait for the server to shutdown
StaticFiles="static"           # Where ALL static files go
HeaderLimit=10000              # Maximum allowed header size on POST
Timeout="30s"                  # How long a connection is allowed to last
Database="data/content.db"     # Path to the contentapi database file
Uploads="data/uploads"         # Path to the contentapi uploads (images)
Templates="static/templates"   # Path to all the templates
LoginCookie="gontentapi_login" # Name of login cookie
LoginExpire="1500h"            # How long the login cookie lasts
MaxSessions=10000              # How many total sessions can exist
# SessionTrim=0.5                # How much to trim the sessions by when it gets too full

# MUST set to empty path if hosted at root!
RootPath=""                   # Root path for our service. Useful when running behind a reverse proxy
`
	return fmt.Sprintf(
		baseConfig,
		time.Now().Format(time.RFC3339),
	)
}
