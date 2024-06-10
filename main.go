package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"slices"
	//"sync"
	"syscall"
	"time"

	"github.com/chi-middleware/proxy"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/pelletier/go-toml/v2"

	"github.com/randomouscrap98/gontentapi/utils"
)

const (
	ConfigFile = "config.toml"
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func initConfig() *Config {
	var config Config
	results, err := utils.ReadConfigStack(ConfigFile, func(_ string, raw []byte) error {
		return toml.Unmarshal(raw, &config)
	}, 10)
	must(err)
	// This means the original config wasn't loaded; let's load it and try again
	if slices.Index(results, ConfigFile) < 0 {
		configRaw := GetDefaultConfig_Toml()
		must(os.WriteFile(ConfigFile, []byte(configRaw), 0600))
		log.Printf("Generated default config at %s\n", ConfigFile)
		return initConfig() // Redo the configs (it's fine...?)
	}
	log.Printf("Loaded %d config(s)", len(results))
	return &config
}

func initRouter(config *Config) *chi.Mux {
	r := chi.NewRouter()

	r.Use(proxy.ForwardedHeaders())
	r.Use(middleware.Logger)
	r.Use(cors.AllowAll().Handler)
	r.Use(middleware.Timeout(time.Duration(config.Timeout)))
	//r.Use(httprate.LimitByIP(config.RateLimitCount, time.Duration(config.RateLimitInterval)))

	return r
}

// Initialize and spawn the http server for the given handler and with the given config
func runServer(handler http.Handler, config *Config) *http.Server {
	s := &http.Server{
		Addr:           config.Address,
		Handler:        handler,
		MaxHeaderBytes: config.HeaderLimit,
	}

	go func() {
		log.Printf("Running server in goroutine at %s", s.Addr)
		if err := s.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("ListenAndServe: %v", err)
		}
	}()

	return s
}

// Great readup: https://dev.to/mokiat/proper-http-shutdown-in-go-3fji
func waitForSigterm() {
	// Create a channel to listen for OS signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)

	// Block until a signal is received
	<-sigChan
}

func main() {
	log.Printf("Gontentapi server started\n")
	config := initConfig()

	gctx, err := NewContext(config)
	must(err)

	// Context is something we'll cancel to cancel any and all background tasks
	// when the server gets a shutdown signal. This for some reason does not
	// include the server itself...
	// _, cancel := context.WithCancel(context.Background())
	// defer cancel()

	r := initRouter(config)
	err = SetupRoutes(r, gctx)
	must(err)

	// var wg sync.WaitGroup

	//func (mctx *MakaiContext) GetHandler() (http.Handler, error) {

	// --- Host all services ---
	// r.Mount(k, handler)
	// wg.Add(1)
	// service.RunBackground(ctx, &wg)
	// log.Printf("Mounted '%s' at %s", service.GetIdentifier(), k)

	// --- Static files -----
	//must(err)

	// --- Server ---
	s := runServer(r, config)
	waitForSigterm()

	log.Println("Shutting down...")
	//cancel() // Cancel the context to signal goroutines to stop
	//wg.Wait()
	log.Println("All background services stopped")

	// Create a context with a timeout to allow for graceful shutdown
	ctxShutdown, cancelShutdown := context.WithTimeout(context.Background(), time.Duration(config.ShutdownTime))
	defer cancelShutdown()

	// Shut down the server gracefully
	if err := s.Shutdown(ctxShutdown); err != nil {
		log.Fatalf("Server shutdown failed: %v", err)
	}

	log.Println("Server stopped")
}
