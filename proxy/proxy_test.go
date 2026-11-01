package main

import (
	"bytes"
	"encoding/json"
	"image"
	"image/draw"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func testProxy(t *testing.T, h http.HandlerFunc) *Proxy {
	t.Helper()
	up := httptest.NewServer(h)
	t.Cleanup(up.Close)
	u, _ := url.Parse(up.URL)
	public, _ := url.Parse("http://proxy:18080")
	return newProxy(Config{Upstream: u, Public: public, RewriteOrigins: []string{up.URL, "http://old-stash:9999"}, APIKey: "test-secret", Workers: 2, Timeout: 3 * time.Second, CacheBytes: 1 << 20, CacheTTL: time.Minute})
}
func request(p *Proxy, method, path string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	p.ServeHTTP(w, httptest.NewRequest(method, path, nil))
	return w
}
func TestRoutingAndCacheIsolation(t *testing.T) {
	var downloads, crops atomic.Int32
	p := testProxy(t, func(w http.ResponseWriter, r *http.Request) {
		downloads.Add(1)
		if r.Header.Get("ApiKey") != "test-secret" {
			t.Error("authentication not injected")
		}
		if strings.Contains(r.URL.RawQuery, "emby_poster") {
			t.Error("marker forwarded to Stash")
		}
		if r.URL.Query().Get("sig") != "a+b/=" {
			t.Error("signature query damaged")
		}
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("original"))
	})
	p.crop = func(b []byte) (ImageResult, error) {
		crops.Add(1)
		return ImageResult{Body: []byte("cropped"), ContentType: "image/jpeg", Mode: "auto", Position: .25}, nil
	}
	path := "/scene/12/screenshot?t=1&sig=a%2Bb%2F%3D"
	for i := 0; i < 2; i++ {
		w := request(p, "GET", path+"&emby_poster=1")
		if w.Body.String() != "cropped" || w.Header().Get("X-Emby-Poster") != "auto" {
			t.Fatal("poster not cropped")
		}
	}
	if crops.Load() != 1 || downloads.Load() != 1 {
		t.Fatal("poster cache miss")
	}
	if w := request(p, "GET", path); w.Body.String() != "original" {
		t.Fatal("backdrop polluted by poster")
	}
	_ = request(p, "GET", strings.Replace(path, "t=1", "t=2", 1)+"&emby_poster=1")
	if crops.Load() != 2 {
		t.Fatal("changed source version reused stale crop")
	}
}
func TestJSONAndRedirectRewrites(t *testing.T) {
	var p *Proxy
	p = testProxy(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redirect" {
			http.Redirect(w, r, p.config.Upstream.String()+"/scene/1/screenshot", 302)
			return
		}
		w.Header().Set("Content-Type", "application/graphql-response+json")
		_, _ = io.WriteString(w, `{"data":{"url":"http:\/\/old-stash:9999\/scene\/1\/screenshot?t=1","id":9007199254740993,"note":"keep http://old-stash:9999 inside text"}}`)
	})
	w := request(p, "POST", "/graphql")
	if w.Code != 200 {
		t.Fatal(w.Code)
	}
	if !strings.Contains(w.Body.String(), "http://proxy:18080/scene/1/screenshot?t=1") || !strings.Contains(w.Body.String(), "9007199254740993") || !strings.Contains(w.Body.String(), "keep http://old-stash:9999 inside text") {
		t.Fatal("JSON rewrite damaged response", w.Body.String())
	}
	if loc := request(p, "GET", "/redirect").Header().Get("Location"); loc != "http://proxy:18080/scene/1/screenshot" {
		t.Fatal(loc)
	}
}
func TestCropFailureReturnsOriginal(t *testing.T) {
	p := testProxy(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = io.WriteString(w, "invalid-png")
	})
	w := request(p, "GET", "/scene/1/screenshot?emby_poster=1")
	if w.Code != 200 || w.Body.String() != "invalid-png" || w.Header().Get("X-Emby-Poster") != "original-error" {
		t.Fatal("fallback failed")
	}
}
func TestActualCropThroughProxy(t *testing.T) {
	img := canvas()
	face := testFace(t)
	draw.Draw(img, image.Rect(40, 100, 160, 230), face, face.Bounds().Min, draw.Src)
	original := imageBytes(t, img)
	p := testProxy(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(original)
	})
	w := request(p, "GET", "/scene/1/screenshot?emby_poster=1")
	if bytes.Equal(w.Body.Bytes(), original) || w.Header().Get("X-Emby-Poster") != "auto" {
		t.Fatal("actual detector not used by proxy")
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(w.Body.Bytes()))
	if err != nil || cfg.Width != 266 || cfg.Height != 400 {
		t.Fatal("wrong output dimensions")
	}
}
func TestExternalRedirectNotFollowed(t *testing.T) {
	var accessed atomic.Bool
	external := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { accessed.Store(true) }))
	defer external.Close()
	p := testProxy(t, func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, external.URL+"/image", 302) })
	if w := request(p, "GET", "/scene/1/screenshot?emby_poster=1"); w.Code != 302 {
		t.Fatal(w.Code)
	}
	if accessed.Load() {
		t.Fatal("proxy followed redirect with Stash credentials")
	}
}
func TestOnlySceneScreenshotCrops(t *testing.T) {
	p := testProxy(t, func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, "original") })
	p.crop = func([]byte) (ImageResult, error) { t.Error("unexpected crop"); return ImageResult{}, nil }
	for _, path := range []string{"/performer/1/image?emby_poster=1", "/graphql?emby_poster=1", "/scene/1/screenshot?emby_poster=0"} {
		if request(p, "GET", path).Body.String() != "original" {
			t.Fatal(path)
		}
	}
}
func TestHeadAndConditionalPoster(t *testing.T) {
	p := testProxy(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.Header.Get("Range") != "" || r.Header.Get("If-None-Match") != "" {
			t.Error("processing request must fetch complete original")
		}
		_, _ = io.WriteString(w, "source")
	})
	p.crop = func([]byte) (ImageResult, error) {
		return ImageResult{Body: []byte("poster"), ContentType: "image/jpeg", Mode: "auto"}, nil
	}
	w := request(p, "HEAD", "/scene/1/screenshot?emby_poster=1")
	if w.Body.Len() != 0 || w.Header().Get("Content-Length") != "6" {
		t.Fatal("HEAD broken")
	}
	r := httptest.NewRequest("GET", "/scene/1/screenshot?emby_poster=1", nil)
	r.Header.Set("If-None-Match", w.Header().Get("ETag"))
	w = httptest.NewRecorder()
	p.ServeHTTP(w, r)
	if w.Code != 304 || w.Body.Len() != 0 {
		t.Fatal("conditional response broken")
	}
}
func TestBusyFallsBackAndNoStoreBypassesCache(t *testing.T) {
	var reads atomic.Int32
	p := testProxy(t, func(w http.ResponseWriter, r *http.Request) {
		reads.Add(1)
		w.Header().Set("Cache-Control", "no-store")
		_, _ = io.WriteString(w, "source")
	})
	p.workers <- struct{}{}
	p.workers <- struct{}{}
	w := request(p, "GET", "/scene/1/screenshot?emby_poster=1")
	if w.Header().Get("X-Emby-Poster") != "original-busy" || w.Body.String() != "source" {
		t.Fatal("busy fallback broken")
	}
	<-p.workers
	<-p.workers
	p.crop = func([]byte) (ImageResult, error) { return ImageResult{Body: []byte("poster"), Mode: "auto"}, nil }
	w = request(p, "GET", "/scene/1/screenshot?emby_poster=1")
	if w.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatal("no-store lost")
	}
	request(p, "GET", "/scene/1/screenshot?emby_poster=1")
	if reads.Load() != 3 {
		t.Fatal("no-store response cached")
	}
}
func TestMarkerRemovalPreservesQueries(t *testing.T) {
	got, marked := removeMarker("t=1&sig=a%2Bb%2F%3D&%65mby_poster=1&EMBY_POSTER=0&x=a+b")
	if !marked || got != "t=1&sig=a%2Bb%2F%3D&x=a+b" {
		t.Fatal("query changed", got)
	}
}
func TestHealthDoesNotCallUpstream(t *testing.T) {
	p := testProxy(t, func(w http.ResponseWriter, r *http.Request) { t.Error("health check hit upstream") })
	if w := request(p, "GET", "/healthz"); w.Code != 200 || w.Body.String() != "ok\n" {
		t.Fatal(w.Code)
	}
}
func TestConfigXMLAndValidation(t *testing.T) {
	t.Setenv("STASH_UPSTREAM", "http://stash:9999")
	t.Setenv("PROXY_PUBLIC_URL", "http://proxy:18080")
	t.Setenv("STASH_API_KEY", "test")
	for _, name := range []string{"STASH_API_KEY_FILE", "EMBY_STASH_CONFIG_FILE", "CACHE_MAX_MB", "CACHE_TTL", "UPSTREAM_TIMEOUT", "CROP_WORKERS", "STASH_REWRITE_ORIGINS"} {
		t.Setenv(name, "")
	}
	if _, err := loadConfig(); err != nil {
		t.Fatal(err)
	}
	xmlPath := filepath.Join(t.TempDir(), "stash.xml")
	if err := os.WriteFile(xmlPath, []byte(`<PluginConfiguration><StashAPIKey>key&amp;encoded</StashAPIKey></PluginConfiguration>`), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EMBY_STASH_CONFIG_FILE", xmlPath)
	if c, err := loadConfig(); err != nil || c.APIKey != "key&encoded" {
		t.Fatal("XML key not decoded")
	}
	t.Setenv("STASH_UPSTREAM", "http://user:password@stash:9999")
	if _, err := loadConfig(); err == nil {
		t.Fatal("credential-bearing upstream accepted")
	}
}
func TestRewriteDoesNotModifyNonURLString(t *testing.T) {
	p := testProxy(t, func(w http.ResponseWriter, r *http.Request) {})
	x := p.rewriteJSON(map[string]any{"url": "http://old-stash:9999.evil/image"})
	b, _ := json.Marshal(x)
	if !strings.Contains(string(b), "old-stash:9999.evil") {
		t.Fatal("origin prefix boundary broken")
	}
}
