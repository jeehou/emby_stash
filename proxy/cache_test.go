package main

import (
	"testing"
	"time"
)

func TestCacheEvictionAndExpiry(t *testing.T) {
	c := newCache(6, time.Minute)
	c.put("a", ImageResult{Body: []byte("aaa")})
	c.put("b", ImageResult{Body: []byte("bbb")})
	if _, ok := c.get("a"); !ok {
		t.Fatal("cache miss")
	}
	c.put("c", ImageResult{Body: []byte("ccc")})
	if _, ok := c.get("b"); ok {
		t.Fatal("LRU entry not evicted")
	}
	c.mu.Lock()
	e := c.items["a"]
	v := e.Value.(cachedImage)
	v.expires = time.Now().Add(-time.Second)
	e.Value = v
	c.mu.Unlock()
	if _, ok := c.get("a"); ok {
		t.Fatal("expired entry returned")
	}
	c.put("large", ImageResult{Body: []byte("1234567")})
	if _, ok := c.get("large"); ok {
		t.Fatal("oversized entry cached")
	}
	if c.used > c.limit {
		t.Fatal("cache exceeded byte limit")
	}
}
