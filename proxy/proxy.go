package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

var screenshotPath = regexp.MustCompile(`^/scene/[0-9]+/screenshot/?$`)

type Proxy struct {
	config  Config
	reverse *httputil.ReverseProxy
	client  *http.Client
	cache   *imageCache
	workers chan struct{}
	crop    CropFunc
}

func newProxy(c Config) *Proxy {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.ResponseHeaderTimeout = c.Timeout
	transport.DisableCompression = true
	p := &Proxy{config: c, cache: newCache(c.CacheBytes, c.CacheTTL), workers: make(chan struct{}, c.Workers), crop: cropPoster}
	p.client = &http.Client{Transport: transport, Timeout: c.Timeout, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	p.reverse = &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			r.SetURL(c.Upstream)
			r.Out.Host = c.Upstream.Host
			r.Out.Header.Set("ApiKey", c.APIKey)
			r.Out.Header.Set("Accept-Encoding", "identity")
		},
		Transport:      transport,
		ModifyResponse: p.rewriteResponse,
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			http.Error(w, "Stash upstream unavailable", http.StatusBadGateway)
		},
		// Standard reverse-proxy errors can contain credential-bearing URLs.
		ErrorLog: log.New(io.Discard, "", 0),
	}
	return p
}

func removeMarker(raw string) (string, bool) {
	parts := strings.Split(raw, "&")
	out := make([]string, 0, len(parts))
	marked := false
	for _, part := range parts {
		key, val, _ := strings.Cut(part, "=")
		decoded, e := url.QueryUnescape(key)
		if e == nil && strings.EqualFold(decoded, "emby_poster") {
			v, _ := url.QueryUnescape(val)
			marked = marked || v == "1"
			continue
		}
		out = append(out, part)
	}
	return strings.Join(out, "&"), marked
}

func (p *Proxy) rewriteURL(s string) string {
	for _, origin := range p.config.RewriteOrigins {
		if s == origin || strings.HasPrefix(s, origin+"/") {
			return p.config.Public.String() + strings.TrimPrefix(s, origin)
		}
	}
	return s
}
func (p *Proxy) rewriteJSON(v any) any {
	switch x := v.(type) {
	case string:
		return p.rewriteURL(x)
	case []any:
		for i := range x {
			x[i] = p.rewriteJSON(x[i])
		}
	case map[string]any:
		for k := range x {
			x[k] = p.rewriteJSON(x[k])
		}
	}
	return v
}
func (p *Proxy) rewriteResponse(r *http.Response) error {
	if s := r.Header.Get("Location"); s != "" {
		r.Header.Set("Location", p.rewriteURL(s))
	}
	t := strings.ToLower(r.Header.Get("Content-Type"))
	if !strings.Contains(t, "application/json") && !strings.Contains(t, "application/graphql-response+json") {
		return nil
	}
	defer r.Body.Close()
	b, e := io.ReadAll(io.LimitReader(r.Body, (32<<20)+1))
	if e != nil || len(b) > 32<<20 {
		return errors.New("JSON response exceeds limit or cannot be read")
	}
	// Preserve large numeric IDs by avoiding float64 decoding.
	var v any
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	if e = dec.Decode(&v); e != nil {
		return errors.New("invalid upstream JSON")
	}
	b, e = json.Marshal(p.rewriteJSON(v))
	if e != nil {
		return errors.New("cannot rewrite JSON")
	}
	r.Body = io.NopCloser(bytes.NewReader(b))
	r.ContentLength = int64(len(b))
	r.Header.Set("Content-Length", strconv.Itoa(len(b)))
	r.Header.Del("ETag")
	r.Header.Del("Content-MD5")
	return nil
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/healthz" {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = io.WriteString(w, "ok\n")
		return
	}
	clean := r.Clone(r.Context())
	u := *r.URL
	clean.URL = &u
	query, marked := removeMarker(r.URL.RawQuery)
	clean.URL.RawQuery = query
	if !marked || !screenshotPath.MatchString(r.URL.Path) || (r.Method != "GET" && r.Method != "HEAD") {
		p.reverse.ServeHTTP(w, clean)
		return
	}
	keyBytes := sha256.Sum256([]byte(p.config.Upstream.String() + clean.URL.RequestURI() + algorithmVersion))
	key := hex.EncodeToString(keyBytes[:])
	if v, ok := p.cache.get(key); ok {
		p.writeImage(w, r, v)
		return
	}
	select {
	case p.workers <- struct{}{}:
		defer func() { <-p.workers }()
	default:
		w.Header().Set("X-Emby-Poster", "original-busy")
		p.reverse.ServeHTTP(w, clean)
		return
	}
	target := *clean.URL
	target.Scheme = p.config.Upstream.Scheme
	target.Host = p.config.Upstream.Host
	req, err := http.NewRequestWithContext(r.Context(), "GET", target.String(), nil)
	if err != nil {
		http.Error(w, "invalid image request", 400)
		return
	}
	req.Header = clean.Header.Clone()
	req.Header.Set("ApiKey", p.config.APIKey)
	req.Header.Set("Accept-Encoding", "identity")
	for _, h := range []string{"Range", "If-Range", "If-None-Match", "If-Modified-Since", "If-Match", "If-Unmodified-Since"} {
		req.Header.Del(h)
	}
	res, err := p.client.Do(req)
	if err != nil {
		http.Error(w, "Stash image unavailable", 502)
		return
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		p.copyOriginal(w, r, res, res.Body, "")
		return
	}
	b, err := io.ReadAll(io.LimitReader(res.Body, maxImageBytes+1))
	if err != nil {
		http.Error(w, "cannot read Stash image", 502)
		return
	}
	if len(b) > maxImageBytes {
		p.copyOriginal(w, r, res, io.MultiReader(bytes.NewReader(b), res.Body), "original-too-large")
		return
	}
	v, err := p.crop(b)
	if err != nil {
		v = ImageResult{Body: b, Mode: "original-error"}
	}
	if v.ContentType == "" {
		v.ContentType = res.Header.Get("Content-Type")
		if v.ContentType == "" {
			v.ContentType = http.DetectContentType(v.Body)
		}
	}
	cc := strings.ToLower(res.Header.Get("Cache-Control"))
	v.NoStore = strings.Contains(cc, "no-store") || res.Header.Get("Set-Cookie") != ""
	if !v.NoStore && !strings.Contains(cc, "no-cache") && v.Mode != "original-error" {
		p.cache.put(key, v)
	}
	p.writeImage(w, r, v)
}

func (p *Proxy) writeImage(w http.ResponseWriter, r *http.Request, v ImageResult) {
	h := sha256.Sum256(v.Body)
	tag := fmt.Sprintf(`"%x"`, h)
	w.Header().Set("ETag", tag)
	w.Header().Set("Content-Type", v.ContentType)
	// Let the bounded internal cache handle reuse; browsers revalidate each time.
	w.Header().Set("Cache-Control", "private, no-cache")
	if v.NoStore {
		w.Header().Set("Cache-Control", "private, no-store")
	}
	w.Header().Set("X-Emby-Poster", v.Mode)
	if v.Mode == "auto" {
		w.Header().Set("X-Emby-Poster-Position", strconv.FormatFloat(v.Position, 'f', 4, 64))
		if v.Strategy != "" {
			w.Header().Set("X-Emby-Poster-Strategy", v.Strategy)
		}
	}
	if r.Header.Get("If-None-Match") == tag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.Header().Set("Content-Length", strconv.Itoa(len(v.Body)))
	if r.Method != "HEAD" {
		_, _ = w.Write(v.Body)
	}
}
func (p *Proxy) copyOriginal(w http.ResponseWriter, r *http.Request, res *http.Response, body io.Reader, mode string) {
	for k, vs := range res.Header {
		for _, v := range vs {
			w.Header().Add(k, v)
		}
	}
	if loc := res.Header.Get("Location"); loc != "" {
		w.Header().Set("Location", p.rewriteURL(loc))
	}
	if mode != "" {
		w.Header().Set("X-Emby-Poster", mode)
	}
	w.WriteHeader(res.StatusCode)
	if r.Method != "HEAD" {
		_, _ = io.Copy(w, body)
	}
}
