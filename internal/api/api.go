package api

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"sciplayer-api/internal/store"
)

type API struct {
	store  store.Store
	logger *log.Logger
	mux    *http.ServeMux
}

type deviceRequest struct {
	DeviceID string `json:"deviceId"`
}

type playlistRequest struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type playlistResponse struct {
	Name      string    `json:"name"`
	URL       string    `json:"url"`
	CreatedAt time.Time `json:"createdAt"`
}

func New(s store.Store, logger *log.Logger) http.Handler {
	if logger == nil {
		logger = log.New(os.Stdout, "sciplayer-api ", log.LstdFlags|log.LUTC)
	}

	api := &API{
		store:  s,
		logger: logger,
	}
	api.mux = api.buildMux()

	return api
}

func (a *API) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	a.mux.ServeHTTP(w, r)
	a.logger.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
}

func (a *API) buildMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", a.handleHealthz)
	mux.HandleFunc("/devices", a.handleDevices)
	mux.HandleFunc("/devices/", a.handleDeviceSubroutes)

	return mux
}

func (a *API) handleHealthz(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		a.methodNotAllowed(w, http.MethodGet)
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func (a *API) handleDevices(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		a.createDevice(w, r)
	default:
		a.methodNotAllowed(w, http.MethodPost)
	}
}

func (a *API) handleDeviceSubroutes(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/devices/")
	segments := strings.Split(path, "/")

	if len(segments) < 1 || segments[0] == "" {
		http.NotFound(w, r)
		return
	}

	deviceID := segments[0]

	if len(segments) == 1 {
		http.NotFound(w, r)
		return
	}

	switch segments[1] {
	case "playlists":
		a.handlePlaylists(w, r, deviceID)
	default:
		http.NotFound(w, r)
	}
}

func (a *API) createDevice(w http.ResponseWriter, r *http.Request) {
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {

		}
	}(r.Body)

	var req deviceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		a.badRequest(w, "invalid JSON payload")
		return
	}

	req.DeviceID = strings.TrimSpace(req.DeviceID)
	if req.DeviceID == "" {
		a.badRequest(w, "deviceId is required")
		return
	}

	created, err := a.store.CreateDevice(r.Context(), req.DeviceID)
	if err != nil {
		a.internalServerError(w, err)
		return
	}

	status := http.StatusCreated
	if !created {
		status = http.StatusOK
	}

	a.respondJSON(w, status, map[string]any{
		"deviceId": req.DeviceID,
		"created":  created,
	})
}

func (a *API) handlePlaylists(w http.ResponseWriter, r *http.Request, deviceID string) {
	switch r.Method {
	case http.MethodPost:
		a.addPlaylist(w, r, deviceID)
	case http.MethodGet:
		a.listPlaylists(w, r, deviceID)
	default:
		a.methodNotAllowed(w, http.MethodPost, http.MethodGet)
	}
}

func (a *API) addPlaylist(w http.ResponseWriter, r *http.Request, deviceID string) {
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {

		}
	}(r.Body)

	var req playlistRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		a.badRequest(w, "invalid JSON payload")
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	req.URL = strings.TrimSpace(req.URL)

	if req.Name == "" {
		a.badRequest(w, "name is required")
		return
	}

	if req.URL == "" {
		a.badRequest(w, "url is required")
		return
	}

	if err := validateURL(req.URL); err != nil {
		a.badRequest(w, "url must be a valid absolute URL")
		return
	}

	if err := a.store.AddPlaylist(r.Context(), deviceID, req.Name, req.URL); err != nil {
		if errors.Is(err, store.ErrDeviceNotFound) {
			http.Error(w, "device not found", http.StatusNotFound)
			return
		}
		a.internalServerError(w, err)
		return
	}

	a.respondJSON(w, http.StatusCreated, map[string]string{
		"deviceId": deviceID,
		"name":     req.Name,
		"url":      req.URL,
	})
}

func (a *API) listPlaylists(w http.ResponseWriter, r *http.Request, deviceID string) {
	playlists, err := a.store.ListPlaylists(r.Context(), deviceID)
	if err != nil {
		if errors.Is(err, store.ErrDeviceNotFound) {
			http.Error(w, "device not found", http.StatusNotFound)
			return
		}
		a.internalServerError(w, err)
		return
	}

	resp := make([]playlistResponse, 0, len(playlists))
	for _, pl := range playlists {
		resp = append(resp, playlistResponse{
			Name:      pl.Name,
			URL:       pl.URL,
			CreatedAt: pl.CreatedAt,
		})
	}

	a.respondJSON(w, http.StatusOK, resp)
}

func (a *API) respondJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		a.logger.Printf("failed to encode response: %v", err)
	}
}

func (a *API) badRequest(w http.ResponseWriter, message string) {
	a.respondJSON(w, http.StatusBadRequest, map[string]string{"error": message})
}

func (a *API) internalServerError(w http.ResponseWriter, err error) {
	a.respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	a.logger.Printf("internal error: %v", err)
}

func (a *API) methodNotAllowed(w http.ResponseWriter, allowedMethods ...string) {
	w.Header().Set("Allow", strings.Join(allowedMethods, ", "))
	a.respondJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
}

func validateURL(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil {
		return err
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return errors.New("url must be absolute")
	}
	return nil
}
