package server

import (
	"job-applications/internal/database"
	"job-applications/internal/templates"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httprate"
)

const (
	requestLimit = 100
	windowLength = time.Minute
)

type Config struct {
	Port              string
	IdleTimeout       time.Duration
	ReadHeaderTimeout time.Duration
}

func New(cfg Config, db *database.DB, renderer *templates.BaseRenderer) *http.Server {
	r := chi.NewRouter()
	r.Use(middleware.ClientIPFromRemoteAddr)
	r.Use(httprate.LimitBy(requestLimit, windowLength, func(r *http.Request) (string, error) {
		return httprate.CanonicalizeIP(middleware.GetClientIP(r.Context())), nil
	}))
	setupRoutes(r, db, renderer)

	return &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           r,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		IdleTimeout:       cfg.IdleTimeout,
	}
}
