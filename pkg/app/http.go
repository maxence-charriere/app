package app

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/maxence-charriere/go-app/v9/pkg/errors"
)

const (
	defaultThemeColor         = "#2d2c2c"
	defaultPreRenderCacheSize = 8000000
	defaultPreRenderCacheTTL  = time.Hour * 24
)

// Handler is an HTTP handler that serves an HTML page that loads a Go wasm app
// and its resources.
type Handler struct {
	// The page authors.
	Author string

	// A placeholder background color for the application page to display before
	// its stylesheets are loaded.
	//
	// DEFAULT: #2d2c2c.
	BackgroundColor string

	// The path of the static resources that the browser is caching in order to
	// provide offline mode.
	//
	// Note that Icon, Styles and Scripts are already cached by default.
	//
	// Paths are relative to the root directory.
	CacheableResources []string

	// The page description.
	Description string

	// The environment variables that are passed to the progressive web app.
	//
	// Reserved keys:
	// - GOAPP_VERSION
	// - GOAPP_GOAPP_STATIC_RESOURCES_URL
	Env Environment

	// The icon that is used for the PWA, favicon, loading and default not
	// found component.
	Icon Icon

	// The path of the default image that is used by social networks when
	// linking the app.
	Image string

	// The URLs that are launched in the app tab or window.
	//
	// By default, URLs with a different domain are launched in another tab.
	// Specifying internal URLs is to override that behavior. A good use case
	// would be the URL for an OAuth authentication.
	InternalURLs []string

	// The page keywords.
	Keywords []string

	// The text displayed while loading a page.
	LoadingLabel string

	// The name of the web application as it is usually displayed to the user.
	Name string

	// Customize the body element before mounting
	OnBody func(body HTMLBody)

	// The cache that stores pre-rendered pages.
	//
	// Default is a LRU cache that keeps pages up to 24h and have a maximum size
	// of 8MB.
	PreRenderCache PreRenderCache

	// The static resources that are accessible from custom paths. Files that
	// are proxied by default are /robots.txt, /sitemap.xml and /ads.txt.
	ProxyResources []ProxyResource

	// Additional headers to be added in head element.
	RawHeaders []string

	// The paths or urls of the JavaScript files to use with the page.
	//
	// eg:
	//  app.Handler{
	//      Scripts: []string{
	//          "/web/test.js",            // Static resource
	//          "https://foo.com/test.js", // External resource
	//      },
	//  },
	Scripts []string

	// The name of the web application displayed to the user when there is not
	// enough space to display Name.
	ShortName string

	// The resource provider that provides static resources. Static resources
	// are always accessed from a path that starts with "/web/".
	//
	// eg:
	//  "/web/main.css"
	//
	// Default: LocalDir("")
	Resources ResourceProvider

	// The paths or urls of the CSS files to use with the page.
	//
	// eg:
	//  app.Handler{
	//      Styles: []string{
	//          "/web/test.css",            // Static resource
	//          "https://foo.com/test.css", // External resource
	//      },
	//  },
	Styles []string

	// The theme color for the application. This affects how the OS displays the
	// app (e.g., PWA title bar or Android's task switcher).
	//
	// DEFAULT: #2d2c2c.
	ThemeColor string

	// The page title.
	Title string

	// The version number. This is used in order to update the PWA application
	// in the browser. It must be set when deployed on a live system in order to
	// prevent recurring updates.
	//
	// Default: Auto-generated in order to trigger pwa update on a local
	// development system.
	Version string

	once           sync.Once
	etag           string
	pwaResources   PreRenderCache
	proxyResources map[string]ProxyResource
}

func (h *Handler) init() {
	h.initVersion()
	h.initStaticResources()
	h.initImage()
	h.initStyles()
	h.initScripts()
	h.initCacheableResources()
	h.initIcon()
	h.initPWA()
	h.initPreRenderedResources()
	h.initProxyResources()
}

func (h *Handler) initVersion() {
	if h.Version == "" {
		t := time.Now().UTC().String()
		h.Version = fmt.Sprintf(`%x`, sha1.Sum([]byte(t)))
	}
	h.etag = `"` + h.Version + `"`
}

func (h *Handler) initStaticResources() {
	if h.Resources == nil {
		h.Resources = LocalDir("")
	}
}

func (h *Handler) initImage() {
	if h.Image != "" {
		h.Image = h.resolveStaticPath(h.Image)
	}
}

func (h *Handler) initStyles() {
	for i, path := range h.Styles {
		h.Styles[i] = h.resolveStaticPath(path)
	}
}

func (h *Handler) initScripts() {
	for i, path := range h.Scripts {
		h.Scripts[i] = h.resolveStaticPath(path)
	}
}

func (h *Handler) initCacheableResources() {
	for i, path := range h.CacheableResources {
		h.CacheableResources[i] = h.resolveStaticPath(path)
	}
}

func (h *Handler) initIcon() {
	if h.Icon.Default == "" {
		h.Icon.Default = "https://storage.googleapis.com/murlok-github/icon-192.png"
		h.Icon.Large = "https://storage.googleapis.com/murlok-github/icon-512.png"
	}

	if h.Icon.AppleTouch == "" {
		h.Icon.AppleTouch = h.Icon.Default
	}

	h.Icon.Default = h.resolveStaticPath(h.Icon.Default)
	h.Icon.Large = h.resolveStaticPath(h.Icon.Large)
	h.Icon.AppleTouch = h.resolveStaticPath(h.Icon.AppleTouch)
}

func (h *Handler) initPWA() {
	if h.Name == "" && h.ShortName == "" && h.Title == "" {
		h.Name = "App PWA"
	}
	if h.ShortName == "" {
		h.ShortName = h.Name
	}
	if h.Name == "" {
		h.Name = h.ShortName
	}

	if h.BackgroundColor == "" {
		h.BackgroundColor = defaultThemeColor
	}
	if h.ThemeColor == "" {
		h.ThemeColor = defaultThemeColor
	}

	if h.LoadingLabel == "" {
		h.LoadingLabel = "Loading"
	}
}

func (h *Handler) initPreRenderedResources() {
	h.pwaResources = newPreRenderCache(5)
	ctx := context.TODO()

	h.pwaResources.Set(ctx, PreRenderedItem{
		Path:        "/wasm_exec.js",
		ContentType: "application/javascript",
		Body:        []byte(wasmExecJS),
	})

	h.pwaResources.Set(ctx, PreRenderedItem{
		Path:        "/app.js",
		ContentType: "application/javascript",
		Body:        h.makeAppJS(),
	})

	h.pwaResources.Set(ctx, PreRenderedItem{
		Path:        "/app-worker.js",
		ContentType: "application/javascript",
		Body:        h.makeAppWorkerJS(),
	})

	h.pwaResources.Set(ctx, PreRenderedItem{
		Path:        "/manifest.webmanifest",
		ContentType: "application/manifest+json",
		Body:        h.makeManifestJSON(),
	})

	h.pwaResources.Set(ctx, PreRenderedItem{
		Path:        "/app.css",
		ContentType: "text/css",
		Body:        []byte(appCSS),
	})

	if h.PreRenderCache == nil {
		h.PreRenderCache = NewPreRenderLRUCache(
			defaultPreRenderCacheSize,
			defaultPreRenderCacheTTL,
		)
	}
}

func (h *Handler) makeAppJS() []byte {
	if h.Env == nil {
		h.Env = make(map[string]string)
	}
	internalURLs, _ := json.Marshal(h.InternalURLs)
	h.Env["GOAPP_INTERNAL_URLS"] = string(internalURLs)
	h.Env["GOAPP_VERSION"] = h.Version
	h.Env["GOAPP_STATIC_RESOURCES_URL"] = h.Resources.Static()
	h.Env["GOAPP_ROOT_PREFIX"] = h.Resources.Package()

	for k, v := range h.Env {
		if err := os.Setenv(k, v); err != nil {
			Log(errors.New("setting app env variable failed").
				Tag("name", k).
				Tag("value", v).
				Wrap(err))
		}
	}

	env, err := json.Marshal(h.Env)
	if err != nil {
		panic(errors.New("encoding pwa env failed").
			Tag("env", h.Env).
			Wrap(err),
		)
	}

	var b bytes.Buffer
	if err := template.
		Must(template.New("app.js").Parse(appJS)).
		Execute(&b, struct {
			Env      string
			Wasm     string
			WorkerJS string
		}{
			Env:      btos(env),
			Wasm:     h.Resources.AppWASM(),
			WorkerJS: h.resolvePackagePath("/app-worker.js"),
		}); err != nil {
		panic(errors.New("initializing app.js failed").Wrap(err))
	}
	return b.Bytes()
}

func (h *Handler) makeAppWorkerJS() []byte {
	cacheableResources := map[string]struct{}{
		h.resolvePackagePath("/app.css"):              {},
		h.resolvePackagePath("/app.js"):               {},
		h.resolvePackagePath("/manifest.webmanifest"): {},
		h.resolvePackagePath("/wasm_exec.js"):         {},
		h.resolvePackagePath("/"):                     {},
		h.Resources.AppWASM():                         {},
	}

	cacheResources := func(res ...string) {
		for _, r := range res {
			if r == "" {
				continue
			}
			cacheableResources[r] = struct{}{}
		}
	}
	cacheResources(h.Icon.Default, h.Icon.Large, h.Icon.AppleTouch)
	cacheResources(h.Styles...)
	cacheResources(h.Scripts...)
	cacheResources(h.CacheableResources...)

	var b bytes.Buffer
	if err := template.
		Must(template.New("app-worker.js").Parse(appWorkerJS)).
		Execute(&b, struct {
			Version          string
			ResourcesToCache map[string]struct{}
		}{
			Version:          h.Version,
			ResourcesToCache: cacheableResources,
		}); err != nil {
		panic(errors.New("initializing app-worker.js failed").Wrap(err))
	}
	return b.Bytes()
}

func (h *Handler) makeManifestJSON() []byte {
	normalize := func(s string) string {
		if !strings.HasPrefix(s, "/") {
			s = "/" + s
		}
		if !strings.HasSuffix(s, "/") {
			s += "/"
		}
		return s
	}

	var b bytes.Buffer
	if err := template.
		Must(template.New("manifest.webmanifest").Parse(manifestJSON)).
		Execute(&b, struct {
			ShortName       string
			Name            string
			Description     string
			DefaultIcon     string
			LargeIcon       string
			BackgroundColor string
			ThemeColor      string
			Scope           string
			StartURL        string
		}{
			ShortName:       h.ShortName,
			Name:            h.Name,
			Description:     h.Description,
			DefaultIcon:     h.Icon.Default,
			LargeIcon:       h.Icon.Large,
			BackgroundColor: h.BackgroundColor,
			ThemeColor:      h.ThemeColor,
			Scope:           normalize(h.Resources.Package()),
			StartURL:        normalize(h.Resources.Package()),
		}); err != nil {
		panic(errors.New("initializing manifest.webmanifest failed").Wrap(err))
	}
	return b.Bytes()
}

func (h *Handler) initProxyResources() {
	resources := make(map[string]ProxyResource)

	for _, r := range h.ProxyResources {
		switch r.Path {
		case "/wasm_exec.js",
			"/goapp.js",
			"/app.js",
			"/app-worker.js",
			"/manifest.json",
			"/manifest.webmanifest",
			"/app.css",
			"/app.wasm",
			"/goapp.wasm",
			"/":
			continue

		default:
			if strings.HasPrefix(r.Path, "/") && strings.HasPrefix(r.ResourcePath, "/web/") {
				resources[r.Path] = r
			}
		}
	}

	if _, ok := resources["/robots.txt"]; !ok {
		resources["/robots.txt"] = ProxyResource{
			Path:         "/robots.txt",
			ResourcePath: "/web/robots.txt",
		}
	}
	if _, ok := resources["/sitemap.xml"]; !ok {
		resources["/sitemap.xml"] = ProxyResource{
			Path:         "/sitemap.xml",
			ResourcePath: "/web/sitemap.xml",
		}
	}
	if _, ok := resources["/ads.txt"]; !ok {
		resources["/ads.txt"] = ProxyResource{
			Path:         "/ads.txt",
			ResourcePath: "/web/ads.txt",
		}
	}

	h.proxyResources = resources
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.once.Do(h.init)

	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("ETag", h.etag)

	etag := r.Header.Get("If-None-Match")
	if etag == h.etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	path := r.URL.Path

	fileHandler, isServingStaticResources := h.Resources.(http.Handler)
	if isServingStaticResources && strings.HasPrefix(path, "/web/") {
		fileHandler.ServeHTTP(w, r)
		return
	}

	switch path {
	case "/goapp.js":
		path = "/app.js"

	case "/manifest.json":
		path = "/manifest.webmanifest"

	case "/app.wasm", "/goapp.wasm":
		if isServingStaticResources {
			r2 := *r
			r2.URL.Path = h.Resources.AppWASM()
			fileHandler.ServeHTTP(w, &r2)
			return
		}

		w.WriteHeader(http.StatusNotFound)
		return

	}

	if res, ok := h.pwaResources.Get(r.Context(), path); ok {
		h.servePreRenderedItem(w, res)
		return
	}

	if res, ok := h.PreRenderCache.Get(r.Context(), path); ok {
		h.servePreRenderedItem(w, res)
		return
	}

	if proxyResource, ok := h.proxyResources[path]; ok {
		h.serveProxyResource(proxyResource, w, r)
		return
	}

	h.servePage(w, r)
}

func (h *Handler) servePreRenderedItem(w http.ResponseWriter, r PreRenderedItem) {
	w.Header().Set("Content-Length", strconv.Itoa(r.Size()))
	w.Header().Set("Content-Type", r.ContentType)
	if r.ContentEncoding != "" {
		w.Header().Set("Content-Encoding", r.ContentEncoding)
	}

	w.WriteHeader(http.StatusOK)
	w.Write(r.Body)
}

func (h *Handler) serveProxyResource(resource ProxyResource, w http.ResponseWriter, r *http.Request) {
	var u string
	if _, ok := h.Resources.(http.Handler); ok {
		u = "http://" + r.Host + resource.ResourcePath
	} else {
		u = h.Resources.Static() + resource.ResourcePath
	}

	res, err := http.Get(u)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		Log(errors.New("getting proxy static resource failed").
			Tag("url", u).
			Tag("proxy-path", resource.Path).
			Tag("static-resource-path", resource.ResourcePath).
			Wrap(err),
		)
		return
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		Log(errors.New("reading proxy static resource failed").
			Tag("url", u).
			Tag("proxy-path", resource.Path).
			Tag("static-resource-path", resource.ResourcePath).
			Wrap(err),
		)
		return
	}

	item := PreRenderedItem{
		Path:            resource.Path,
		ContentType:     res.Header.Get("Content-Type"),
		ContentEncoding: res.Header.Get("Content-Encoding"),
		Body:            body,
	}
	h.PreRenderCache.Set(r.Context(), item)
	h.servePreRenderedItem(w, item)
}

func (h *Handler) servePage(w http.ResponseWriter, r *http.Request) {
	content, ok := routes.createComponent(r.URL.Path)
	if !ok {
		http.NotFound(w, r)
		return
	}

	url := *r.URL
	url.Host = r.Host
	url.Scheme = "http"

	var page requestPage
	page.SetTitle(h.Title)
	page.SetDescription(h.Description)
	page.SetAuthor(h.Author)
	page.SetKeywords(h.Keywords...)
	page.SetLoadingLabel(h.LoadingLabel)
	page.SetImage(h.Image)
	page.url = &url

	disp := engine{
		Page:                   &page,
		RunsInServer:           true,
		ResolveStaticResources: h.resolveStaticPath,
		ActionHandlers:         actionHandlers,
	}
	body := Body().Body(
		Div().Body(
			Aside().
				ID("app-wasm-loader").
				Class("goapp-app-info").
				Body(
					Img().
						ID("app-wasm-loader-icon").
						Class("goapp-logo goapp-spin").
						Src(h.Icon.Default),
					P().
						ID("app-wasm-loader-label").
						Class("goapp-label").
						Text(page.loadingLabel),
				),
			Div().ID("app-pre-render").Body(content),
		),
	)
	if h.OnBody != nil {
		h.OnBody(body)
	}
	if err := mount(&disp, body); err != nil {
		panic(errors.New("mounting pre-rendering container failed").
			Tag("server-side", disp.runsInServer()).
			Tag("body-type", reflect.TypeOf(disp.Body)).
			Wrap(err))
	}
	disp.Body = body
	disp.init()
	defer disp.Close()

	disp.PreRender()

	for len(disp.events) != 0 {
		disp.Consume()
		disp.Wait()
	}

	var b bytes.Buffer
	b.WriteString("<!DOCTYPE html>\n")
	PrintHTML(&b, Html().Body(
		Head().Body(
			Meta().Charset("UTF-8"),
			Meta().
				HTTPEquiv("Content-Type").
				Content("text/html; charset=utf-8"),
			Meta().
				Name("author").
				Content(page.Author()),
			Meta().
				Name("description").
				Content(page.Description()),
			Meta().
				Name("keywords").
				Content(page.Keywords()),
			Meta().
				Name("theme-color").
				Content(h.ThemeColor),
			Meta().
				Name("viewport").
				Content("width=device-width, initial-scale=1, maximum-scale=1, user-scalable=0, viewport-fit=cover"),
			Meta().
				Property("og:url").
				Content(page.URL().String()),
			Meta().
				Property("og:title").
				Content(page.Title()),
			Meta().
				Property("og:description").
				Content(page.Description()),
			Meta().
				Property("og:type").
				Content("website"),
			Meta().
				Property("og:image").
				Content(page.Image()),
			Title().Text(page.Title()),
			Link().
				Rel("icon").
				Type("image/png").
				Href(h.Icon.Default),
			Link().
				Rel("apple-touch-icon").
				Href(h.Icon.AppleTouch),
			Link().
				Rel("manifest").
				Href(h.resolvePackagePath("/manifest.webmanifest")),
			Link().
				Type("text/css").
				Rel("stylesheet").
				Href(h.resolvePackagePath("/app.css")),
			Script().
				Defer(true).
				Src(h.resolvePackagePath("/wasm_exec.js")),
			Script().
				Defer(true).
				Src(h.resolvePackagePath("/app.js")),
			Range(h.Styles).Slice(func(i int) UI {
				return Link().
					Type("text/css").
					Rel("stylesheet").
					Href(h.Styles[i])
			}),
			Range(h.Scripts).Slice(func(i int) UI {
				return Script().
					Defer(true).
					Src(h.Scripts[i])
			}),
			Range(h.RawHeaders).Slice(func(i int) UI {
				return Raw(h.RawHeaders[i])
			}),
		),
		body,
	))

	item := PreRenderedItem{
		Path:        page.URL().Path,
		Body:        b.Bytes(),
		ContentType: "text/html",
	}
	h.PreRenderCache.Set(r.Context(), item)
	h.servePreRenderedItem(w, item)
}

func (h *Handler) resolvePackagePath(path string) string {
	var b strings.Builder

	b.WriteByte('/')
	appResources := strings.Trim(h.Resources.Package(), "/")
	b.WriteString(appResources)

	path = strings.Trim(path, "/")
	if b.Len() != 1 && path != "" {
		b.WriteByte('/')
	}
	b.WriteString(path)

	return b.String()
}

func (h *Handler) resolveStaticPath(path string) string {
	if isRemoteLocation(path) || !isStaticResourcePath(path) {
		return path
	}

	var b strings.Builder
	staticResources := strings.TrimSuffix(h.Resources.Static(), "/")
	b.WriteString(staticResources)
	path = strings.Trim(path, "/")
	b.WriteByte('/')
	b.WriteString(path)
	return b.String()
}

// Icon describes a square image that is used in various places such as
// application icon, favicon or loading icon.
type Icon struct {
	// The path or url to a square image/png file. It must have a side of 192px.
	//
	// Path is relative to the root directory.
	Default string

	// The path or url to larger square image/png file. It must have a side of
	// 512px.
	//
	// Path is relative to the root directory.
	Large string

	// The path or url to a square image/png file that is used for IOS/IPadOS
	// home screen icon. It must have a side of 192px.
	//
	// Path is relative to the root directory.
	//
	// DEFAULT: Icon.Default
	AppleTouch string
}

// Environment describes the environment variables to pass to the progressive
// web app.
type Environment map[string]string

func normalizeFilePath(path string) string {
	if runtime.GOOS == "windows" {
		return strings.ReplaceAll(path, "/", `\`)
	}
	return path
}

func isRemoteLocation(path string) bool {
	return strings.HasPrefix(path, "https://") ||
		strings.HasPrefix(path, "http://")
}

func isStaticResourcePath(path string) bool {
	return strings.HasPrefix(path, "/web/") ||
		strings.HasPrefix(path, "web/")
}

type httpResource struct {
	Path        string
	ContentType string
	Body        []byte
	ExpireAt    time.Time
}

func (r httpResource) Len() int {
	return len(r.Body)
}

func (r httpResource) IsExpired() bool {
	return r.ExpireAt != time.Time{} && r.ExpireAt.Before(time.Now())
}
