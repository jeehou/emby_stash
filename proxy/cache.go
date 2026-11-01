package main

import (
	"container/list"
	"sync"
	"time"
)

type cachedImage struct {
	key     string
	result  ImageResult
	expires time.Time
}
type imageCache struct {
	mu          sync.Mutex
	items       map[string]*list.Element
	lru         *list.List
	used, limit int64
	ttl         time.Duration
}

func newCache(limit int64, ttl time.Duration) *imageCache {
	return &imageCache{items: make(map[string]*list.Element), lru: list.New(), limit: limit, ttl: ttl}
}
func (c *imageCache) remove(e *list.Element) {
	x := e.Value.(cachedImage)
	delete(c.items, x.key)
	c.used -= int64(len(x.result.Body))
	c.lru.Remove(e)
}
func (c *imageCache) get(key string) (ImageResult, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.items[key]
	if !ok {
		return ImageResult{}, false
	}
	v := e.Value.(cachedImage)
	if !time.Now().Before(v.expires) {
		c.remove(e)
		return ImageResult{}, false
	}
	c.lru.MoveToFront(e)
	return v.result, true
}
func (c *imageCache) put(key string, v ImageResult) {
	c.mu.Lock()
	defer c.mu.Unlock()
	size := int64(len(v.Body))
	if size == 0 || size > c.limit {
		return
	}
	if e, ok := c.items[key]; ok {
		c.remove(e)
	}
	for c.used+size > c.limit {
		c.remove(c.lru.Back())
	}
	c.items[key] = c.lru.PushFront(cachedImage{key, v, time.Now().Add(c.ttl)})
	c.used += size
}
