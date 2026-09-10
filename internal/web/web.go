// Package web presents the studio as ordinary HTML forms with optional htmx enhancement.
package web

import (
	"crypto/subtle"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jaminalder/go-graphics/internal/artwork"
	"github.com/jaminalder/go-graphics/internal/publish"
	"github.com/jaminalder/go-graphics/internal/renderjob"
	"github.com/jaminalder/go-graphics/internal/studio"
)

//go:embed templates/*.html assets/*
var files embed.FS

// Config fixes the canonical origin and app dependencies at startup.
type (
	Config struct {
		Origin     string
		Studio     *studio.Store
		Jobs       *renderjob.Manager
		TrustProxy bool
	}
	app struct {
		cfg          Config
		host, cookie string
		secure       bool
		templates    *template.Template
		assets       http.Handler
		hashes       map[string]string
		active       chan struct{}
		limits       limiter
	}
	page struct {
		Title, Kind, Message, CSRF, Action             string
		Entry                                          publish.Entry
		Entries                                        []publish.Entry
		Exploration                                    studio.Exploration
		Samples                                        []studio.Sample
		Sample                                         studio.Sample
		Token                                          string
		Active                                         bool
		Ready                                          int
		Previous                                       []studio.Sample
		BatchID                                        string
		StyleName, ColourName, StyleImage, ColourImage string
		Favourites                                     []studio.Favourite
		Failed                                         bool
	}
)

// New constructs the HTTP handler and validates the publication catalogue without rendering.
func New(c Config) (http.Handler, error) {
	u, e := url.Parse(c.Origin)
	if e != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.Path != "" || u.User != nil {
		return nil, errors.New("invalid canonical origin")
	}
	if c.Studio == nil {
		return nil, errors.New("missing studio")
	}
	if e := publish.Check(); e != nil {
		return nil, e
	}
	hashes, e := catalogueAssets()
	if e != nil {
		return nil, e
	}
	t, e := template.New("page").Funcs(template.FuncMap{"asset": func(name string) string { return "/assets/" + hashes[name] + "/" + name }}).ParseFS(files, "templates/*.html")
	if e != nil {
		return nil, e
	}
	assets, e := fs.Sub(files, "assets")
	if e != nil {
		return nil, e
	}
	a := &app{active: make(chan struct{}, 128), hashes: hashes, cfg: c, host: u.Host, secure: u.Scheme == "https", cookie: "art-studio", templates: t, assets: http.StripPrefix("/assets/", http.FileServer(http.FS(assets))), limits: limiter{keys: map[string]bucket{}}}
	if a.secure {
		a.cookie = "__Host-art-studio"
	}
	return a, nil
}

func (a *app) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h := w.Header()
	h.Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self'; font-src 'self'; connect-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'")
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Referrer-Policy", "same-origin")
	h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), web-share=(self)")
	h.Set("Cache-Control", "private, no-store")
	if r.Host != a.host {
		http.Error(w, "unknown host", 421)
		return
	}
	select {
	case a.active <- struct{}{}:
		defer func() { <-a.active }()
	default:
		a.fail(w, r, 503, "The studio is busy. Try again in a moment.")
		return
	}
	if len(r.RequestURI) > 4096 {
		http.Error(w, "request target too long", http.StatusRequestURITooLong)
		return
	}
	bucketName, rate, burst := "read:", 240.0, 30.0
	if strings.HasPrefix(r.URL.Path, "/assets/") {
		bucketName, rate, burst = "asset:", 1200, 60
	}
	if !a.limits.allow(bucketName+a.client(r), rate, burst) {
		a.fail(w, r, 429, "Too many requests. Give this page a moment.")
		return
	}
	if r.Method == "GET" || r.Method == "HEAD" {
		a.get(w, r)
		return
	}
	if r.Method != "POST" {
		w.Header().Set("Allow", "GET, HEAD, POST")
		http.Error(w, "method unavailable", http.StatusMethodNotAllowed)
		return
	}
	if r.Header.Get("Origin") != a.cfg.Origin {
		a.fail(w, r, 403, "This action must come from this site.")
		return
	}
	if r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
		a.fail(w, r, 415, "This form could not be read.")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 65536)
	if e := r.ParseForm(); e != nil {
		a.fail(w, r, 413, "This form is too large.")
		return
	}
	if len(r.URL.RawQuery) > 0 {
		a.fail(w, r, 400, "Unexpected query values.")
		return
	}
	for k, v := range r.PostForm {
		if len(v) != 1 && k != "selected" {
			a.fail(w, r, 400, "Conflicting form values.")
			return
		}
		if len(v) > 4 {
			a.fail(w, r, 400, "Choose at most four favourites.")
			return
		}
	}
	a.post(w, r)
}

func (a *app) client(r *http.Request) string {
	host, _, e := net.SplitHostPort(r.RemoteAddr)
	if e != nil {
		return "unknown"
	}
	ip, e := netip.ParseAddr(host)
	if e != nil {
		return "unknown"
	}
	if a.cfg.TrustProxy && ip.IsLoopback() {
		if candidate, e := netip.ParseAddr(r.Header.Get("X-Art-Client")); e == nil {
			ip = candidate
		}
	}
	return ip.Unmap().String()
}

func (a *app) session(r *http.Request) (studio.Workspace, error) {
	c, e := r.Cookie(a.cookie)
	if e != nil {
		return studio.Workspace{}, studio.ErrExpired
	}
	return a.cfg.Studio.Session(c.Value)
}

func (a *app) setCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{Name: a.cookie, Value: token, Path: "/", Secure: a.secure, HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: 86400})
}

func (a *app) get(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	if path == "/health/live" {
		w.WriteHeader(204)
		return
	}
	if path == "/health/ready" {
		w.WriteHeader(204)
		return
	}
	if strings.HasPrefix(path, "/assets/") {
		parts := strings.Split(strings.TrimPrefix(path, "/assets/"), "/")
		if len(parts) != 2 || a.hashes[parts[1]] != parts[0] {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		copy := r.Clone(r.Context())
		u := *r.URL
		u.Path = "/assets/" + parts[1]
		copy.URL = &u
		a.assets.ServeHTTP(w, copy)
		return
	}
	if strings.HasPrefix(path, "/images/") || strings.HasPrefix(path, "/downloads/") {
		a.image(w, r)
		return
	}
	if path == "/" {
		a.page(w, r, page{Title: "Find an image of your own", Kind: "gallery", Entries: publish.All()})
		return
	}
	if path == "/about" {
		a.page(w, r, page{Title: "About this studio", Kind: "about"})
		return
	}
	if path == "/recover" {
		http.Redirect(w, r, "/favourites", http.StatusSeeOther)
		return
	}
	if path == "/favourites" {
		p := page{Title: "Favourites", Kind: "favourites"}
		if session, err := a.session(r); err == nil {
			p.CSRF = session.CSRF
			p.Favourites, _ = a.cfg.Studio.Favourites(session.Token)
		}
		a.page(w, r, p)
		return
	}
	if strings.HasPrefix(path, "/art/") {
		entry, ok := publish.Get(strings.TrimPrefix(path, "/art/"))
		if !ok {
			http.NotFound(w, r)
			return
		}
		p := page{Title: entry.Name, Kind: "art", Entry: entry, Action: studio.Token()}
		if s, e := a.session(r); e == nil {
			p.CSRF = s.CSRF
		}
		a.page(w, r, p)
		return
	}
	session, e := a.session(r)
	if e != nil {
		a.fail(w, r, 410, e.Error())
		return
	}
	if path == "/recovery" {
		records, e := a.cfg.Studio.Recovery(session.Token)
		if e != nil {
			a.fail(w, r, 410, e.Error())
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if r.Method != "HEAD" {
			if e := json.NewEncoder(w).Encode(struct {
				Version int               `json:"version"`
				Recipes []json.RawMessage `json:"recipes"`
			}{1, records}); e != nil {
				return
			}
		}
		return
	}
	if path == "/export" {
		records, e := a.cfg.Studio.Recovery(session.Token)
		if e != nil {
			a.fail(w, r, 410, e.Error())
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", `attachment; filename="art-favourites.json"`)
		if e := json.NewEncoder(w).Encode(struct {
			Version int               `json:"version"`
			Recipes []json.RawMessage `json:"recipes"`
		}{1, records}); e != nil {
			return
		}
		return
	}
	fragment := strings.HasPrefix(path, "/fragments/")
	path = strings.TrimPrefix(path, "/fragments")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) < 2 || parts[0] != "explorations" {
		http.NotFound(w, r)
		return
	}
	x, e := a.cfg.Studio.Get(session.Token, parts[1])
	if e != nil {
		a.fail(w, r, 410, e.Error())
		return
	}
	entry, _ := publish.Get(x.Artwork)
	p := page{Title: entry.Name + " studio", Kind: "studio", Exploration: x, Entry: entry, CSRF: session.CSRF, Action: studio.Token(), Active: x.Active}
	if len(x.Batches) > 0 {
		latest := x.Batches[len(x.Batches)-1]
		p.BatchID = latest.ID
		if x.Active && len(x.Batches) > 1 {
			for _, sid := range x.Batches[len(x.Batches)-2].Samples {
				for _, sample := range x.Samples {
					if sample.ID == sid && sample.Status.State == "ready" {
						p.Previous = append(p.Previous, sample)
					}
				}
			}
		}
		for _, id := range latest.Samples {
			for _, sample := range x.Samples {
				if sample.ID == id {
					p.Samples = append(p.Samples, sample)
					if sample.Status.State == "ready" {
						p.Ready++
					}
				}
			}
		}
	}
	if len(parts) == 4 && parts[2] == "samples" {
		found := false
		for _, sample := range x.Samples {
			if sample.ID == parts[3] {
				p.Kind = "sample"
				p.Sample = sample
				found = true
			}
		}
		if !found {
			http.NotFound(w, r)
			return
		}
	} else if len(parts) != 2 {
		http.NotFound(w, r)
		return
	}
	p.StyleName, p.ColourName = "Keep it open", "Surprise me"
	for _, choice := range entry.Styles {
		if choice.ID == x.Style {
			p.StyleName, p.StyleImage = choice.Name, entry.ID+"-"+choice.ID+".png"
		}
	}
	for _, choice := range entry.Colours {
		if choice.ID == x.Colour {
			p.ColourName, p.ColourImage = choice.Name, entry.ID+"-"+choice.ID+".png"
		}
	}
	p.Failed = !p.Active && p.Ready == 0 && len(p.Samples) > 0

	if fragment {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if e := a.templates.ExecuteTemplate(w, "main", p); e != nil {
			return
		}
		return
	}
	a.page(w, r, p)
}

func (a *app) allowed(r *http.Request, keys ...string) bool {
	set := map[string]bool{"csrf": true}
	for _, k := range keys {
		set[k] = true
	}
	for k := range r.PostForm {
		if !set[k] {
			return false
		}
	}
	return true
}

func (a *app) post(w http.ResponseWriter, r *http.Request) {
	session, e := a.session(r)
	fresh := e != nil
	path := r.URL.Path
	if !fresh {
		v := r.PostForm.Get("csrf")
		if subtle.ConstantTimeCompare([]byte(v), []byte(session.CSRF)) != 1 {
			a.fail(w, r, 403, "This form has expired. Refresh the page before trying again.")
			return
		}
	}
	if path == "/explorations" || path == "/restore" {
		if !a.limits.allow("start:"+a.client(r), 12, 2) {
			a.fail(w, r, 429, "Please wait a moment before starting another exploration.")
			return
		}
		if path == "/explorations" && !a.allowed(r, "artwork", "style", "colour", "action") {
			a.fail(w, r, 400, "Unknown choice.")
			return
		}
		if path == "/restore" && !a.allowed(r, "recovery") {
			a.fail(w, r, 400, "Unknown recovery field.")
			return
		}
		if path == "/explorations" {
			if _, _, e := publish.Pins(r.PostForm.Get("artwork"), r.PostForm.Get("style"), r.PostForm.Get("colour")); e != nil {
				a.fail(w, r, 400, e.Error())
				return
			}
		}
		var recovered struct {
			Version int               `json:"version"`
			Recipes []json.RawMessage `json:"recipes"`
		}
		if path == "/restore" {
			if e := artwork.StrictJSON([]byte(r.PostForm.Get("recovery")), &recovered); e != nil || recovered.Version != 1 || len(recovered.Recipes) > 24 {
				a.fail(w, r, 400, "This recovery file is not supported.")
				return
			}
			for _, raw := range recovered.Recipes {
				if _, e := publish.Validate(raw); e != nil {
					a.fail(w, r, 400, "This recovery file contains an unavailable artwork.")
					return
				}
			}
		}
		if fresh {
			session, e = a.cfg.Studio.Create()
			if e != nil {
				a.fail(w, r, 503, e.Error())
				return
			}
			a.setCookie(w, session.Token)
		}
		if path == "/explorations" && !a.allowGeneration(w, r, session.Token) {
			return
		}
		id := ""
		if path == "/restore" {
			id, e = a.cfg.Studio.Restore(session.Token, recovered.Recipes)
		} else {
			id, e = a.cfg.Studio.Enter(session.Token, r.PostForm.Get("artwork"), r.PostForm.Get("style"), r.PostForm.Get("colour"), r.PostForm.Get("action"))
		}
		if e != nil {
			status := 400
			if errors.Is(e, renderjob.ErrBusy) {
				status = 503
			}
			if errors.Is(e, studio.ErrConflict) {
				status = 409
			}
			a.fail(w, r, status, e.Error())
			return
		}
		target := "/"
		if id != "" {
			target = "/explorations/" + id
		}
		http.Redirect(w, r, target, http.StatusSeeOther)
		return
	}
	if fresh {
		a.fail(w, r, 410, studio.ErrExpired.Error())
		return
	}
	if path == "/clear" {
		if !a.allowed(r) {
			a.fail(w, r, 400, "Invalid form.")
			return
		}
		a.cfg.Studio.Clear(session.Token)
		a.setCookie(w, "")
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 3 || parts[0] != "explorations" {
		http.NotFound(w, r)
		return
	}
	id, operation := parts[1], parts[2]
	revision, revisionErr := strconv.Atoi(r.PostForm.Get("revision"))
	if operation != "download" && (revisionErr != nil || revision < 0) {
		a.fail(w, r, 400, "Invalid exploration revision.")
		return
	}
	allowed := false
	switch operation {
	case "choices":
		allowed = a.allowed(r, "revision", "style", "colour")
	case "batches":
		allowed = a.allowed(r, "revision", "action", "batch")
	case "similar":
		allowed = a.allowed(r, "revision", "action", "sample")
	case "favourites":
		allowed = a.allowed(r, "revision", "sample", "on", "return")
	case "download":
		allowed = a.allowed(r, "sample")
	case "cancel":
		allowed = a.allowed(r, "revision", "batch")
	}
	if !allowed {
		a.fail(w, r, 400, "Unknown action field.")
		return
	}
	if operation == "batches" || operation == "similar" || operation == "download" {
		if !a.allowGeneration(w, r, session.Token) {
			return
		}
	}
	target := "/explorations/" + id
	switch operation {
	case "choices":
		e = a.cfg.Studio.Choices(session.Token, id, revision, r.PostForm.Get("style"), r.PostForm.Get("colour"))
	case "batches":
		_, e = a.cfg.Studio.Retry(session.Token, id, revision, r.PostForm.Get("action"), r.PostForm.Get("batch"))
	case "similar":
		_, e = a.cfg.Studio.Generate(session.Token, id, revision, r.PostForm.Get("action"), []string{r.PostForm.Get("sample")}, false)
	case "favourites":
		destination := r.PostForm.Get("return")
		if destination != "" && destination != "sample" && destination != "favourites" {
			a.fail(w, r, 400, "Unknown destination.")
			return
		}
		if destination == "sample" {
			target += "/samples/" + r.PostForm.Get("sample")
		}
		if destination == "favourites" {
			target = "/favourites"
		}
		e = a.cfg.Studio.Favourite(session.Token, id, r.PostForm.Get("sample"), revision, r.PostForm.Get("on") == "yes")
	case "cancel":
		e = a.cfg.Studio.Cancel(session.Token, id, revision, r.PostForm.Get("batch"))
	case "download":
		e = a.cfg.Studio.Download(session.Token, id, r.PostForm.Get("sample"))
		target += "/samples/" + r.PostForm.Get("sample")
	default:
		http.NotFound(w, r)
		return
	}
	if e != nil {
		status := 400
		if errors.Is(e, studio.ErrExpired) {
			status = 410
		}
		if errors.Is(e, studio.ErrConflict) {
			status = 409
		}
		if errors.Is(e, renderjob.ErrBusy) {
			status = 503
		}
		a.fail(w, r, status, e.Error())
		return
	}
	http.Redirect(w, r, target, http.StatusSeeOther)
}

// A burst of three admits create, download, and similar as one natural visit.
// Sustained quotas and the independent queue/worker bounds still cap rendering.
func (a *app) allowGeneration(w http.ResponseWriter, r *http.Request, token string) bool {
	if !a.limits.allow("generate-ip:"+a.client(r), 12, 3) || !a.limits.allow("generate-workspace:"+token, 6, 3) {
		a.fail(w, r, 429, "Give these images a moment, then try again.")
		return false
	}
	return true
}

func (a *app) image(w http.ResponseWriter, r *http.Request) {
	if a.cfg.Jobs == nil {
		http.NotFound(w, r)
		return
	}
	id := strings.TrimPrefix(strings.TrimPrefix(r.URL.Path, "/images/"), "/downloads/")
	if len(id) != 64 {
		http.NotFound(w, r)
		return
	}
	f, digest, e := a.cfg.Jobs.Open(id)
	if errors.Is(e, renderjob.ErrBusy) {
		a.fail(w, r, 503, "Downloads are busy. Try again in a moment.")
		return
	}
	if e != nil {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(410)
		_, _ = io.WriteString(w, "This image has expired. Return to the studio to make it again.")
		return
	}
	defer f.Close()
	info, e := f.Stat()
	if e != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("ETag", `"`+digest+`"`)
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	if strings.HasPrefix(r.URL.Path, "/downloads/") {
		w.Header().Set("Content-Disposition", `attachment; filename="artwork-`+id[:12]+`.png"`)
	}
	http.ServeContent(w, r, "artwork.png", info.ModTime(), f)
}

func (a *app) page(w http.ResponseWriter, r *http.Request, p page) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if r.Method == "HEAD" {
		return
	}
	if e := a.templates.ExecuteTemplate(w, "page", p); e != nil {
		return
	}
}

func (a *app) fail(w http.ResponseWriter, r *http.Request, status int, message string) {
	if status == 429 || status == 503 {
		w.Header().Set("Retry-After", "10")
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	a.page(w, r, page{Title: "A moment to pause", Kind: "error", Message: message})
}

type (
	bucket struct {
		tokens float64
		at     time.Time
	}
	limiter struct {
		mu   sync.Mutex
		keys map[string]bucket
	}
)

func (l *limiter) allow(key string, perMinute, burst float64) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	b, ok := l.keys[key]
	if !ok {
		if len(l.keys) >= 10000 {
			for k, v := range l.keys {
				if now.Sub(v.at) > 5*time.Minute {
					delete(l.keys, k)
				}
			}
			if len(l.keys) >= 10000 {
				return false
			}
		}
		b = bucket{tokens: burst, at: now}
	}
	b.tokens = min(burst, b.tokens+now.Sub(b.at).Seconds()*perMinute/60)
	b.at = now
	allowed := b.tokens >= 1
	if allowed {
		b.tokens--
	}
	l.keys[key] = b
	return allowed
}

// Metrics writes private, low-cardinality queue/cache counters for the operator listener.
func Metrics(jobs *renderjob.Manager) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		q, r, f, b := jobs.Counts()
		_, _ = fmt.Fprintf(w, "art_jobs_queued %d\nart_jobs_running %d\nart_cache_files %d\nart_cache_bytes %d\n", q, r, f, b)
	})
}
