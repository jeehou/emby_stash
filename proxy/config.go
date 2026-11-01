package main

import (
	"encoding/xml"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Listen           string
	Upstream, Public *url.URL
	RewriteOrigins   []string
	APIKey           string
	CacheBytes       int64
	CacheTTL         time.Duration
	Timeout          time.Duration
	Workers          int
}

func origin(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil || u == nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return nil, errors.New("must be an HTTP(S) origin without credentials, query or path")
	}
	u.Path = ""
	return u, nil
}

func loadConfig() (Config, error) {
	c := Config{Listen: "127.0.0.1:18080", CacheBytes: 64 << 20, CacheTTL: 5 * time.Minute, Timeout: 20 * time.Second, Workers: 2}
	if s := os.Getenv("LISTEN_ADDR"); s != "" {
		c.Listen = s
	}
	var err error
	c.Upstream, err = origin(os.Getenv("STASH_UPSTREAM"))
	if err != nil {
		return c, fmt.Errorf("STASH_UPSTREAM: %w", err)
	}
	c.Public, err = origin(os.Getenv("PROXY_PUBLIC_URL"))
	if err != nil {
		return c, fmt.Errorf("PROXY_PUBLIC_URL: %w", err)
	}
	if c.Public.String() == c.Upstream.String() {
		return c, errors.New("upstream and public origins must differ to avoid a proxy loop")
	}
	c.RewriteOrigins = []string{c.Upstream.String()}
	for _, raw := range strings.Split(os.Getenv("STASH_REWRITE_ORIGINS"), ",") {
		if strings.TrimSpace(raw) == "" {
			continue
		}
		u, e := origin(strings.TrimSpace(raw))
		if e != nil {
			return c, fmt.Errorf("STASH_REWRITE_ORIGINS: %w", e)
		}
		c.RewriteOrigins = append(c.RewriteOrigins, u.String())
	}
	c.APIKey = strings.TrimSpace(os.Getenv("STASH_API_KEY"))
	if path := os.Getenv("STASH_API_KEY_FILE"); path != "" {
		b, e := os.ReadFile(path)
		if e != nil {
			return c, errors.New("cannot read STASH_API_KEY_FILE")
		}
		c.APIKey = strings.TrimSpace(string(b))
	} else if path := os.Getenv("EMBY_STASH_CONFIG_FILE"); path != "" {
		b, e := os.ReadFile(path)
		if e != nil {
			return c, errors.New("cannot read EMBY_STASH_CONFIG_FILE")
		}
		var doc struct {
			Key string `xml:"StashAPIKey"`
		}
		if len(b) > 1<<20 || xml.Unmarshal(b, &doc) != nil {
			return c, errors.New("invalid Emby Stash configuration XML")
		}
		c.APIKey = strings.TrimSpace(doc.Key)
	}
	if c.APIKey == "" || strings.ContainsAny(c.APIKey, "\r\n") {
		return c, errors.New("a non-empty Stash API key is required (file, XML or environment)")
	}
	if s := os.Getenv("CACHE_MAX_MB"); s != "" {
		n, e := strconv.Atoi(s)
		if e != nil || n < 0 || n > 4096 {
			return c, errors.New("CACHE_MAX_MB must be 0..4096")
		}
		c.CacheBytes = int64(n) << 20
	}
	if s := os.Getenv("CROP_WORKERS"); s != "" {
		n, e := strconv.Atoi(s)
		if e != nil || n < 1 || n > 16 {
			return c, errors.New("CROP_WORKERS must be 1..16")
		}
		c.Workers = n
	}
	for name, dest := range map[string]*time.Duration{"CACHE_TTL": &c.CacheTTL, "UPSTREAM_TIMEOUT": &c.Timeout} {
		if s := os.Getenv(name); s != "" {
			d, e := time.ParseDuration(s)
			if e != nil || d <= 0 {
				return c, fmt.Errorf("%s must be a positive duration", name)
			}
			*dest = d
		}
	}
	return c, nil
}
