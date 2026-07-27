package cache

import (
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

// LocalCache 本地内存缓存
type LocalCache struct {
	data     sync.Map
	ttl      time.Duration
	cleaner  *time.Ticker
	stopCh   chan struct{}
	stopOnce sync.Once
	group    singleflight.Group
}

type cacheItem struct {
	value      interface{}
	expireTime time.Time
}

// NewLocalCache 创建本地缓存
func NewLocalCache(ttl time.Duration) *LocalCache {
	lc := &LocalCache{
		ttl:     ttl,
		cleaner: time.NewTicker(time.Minute),
		stopCh:  make(chan struct{}),
	}

	go lc.cleanExpired()
	return lc
}

// Get 获取缓存
func (c *LocalCache) Get(key string) (interface{}, bool) {
	if item, ok := c.data.Load(key); ok {
		ci := item.(*cacheItem)
		if time.Now().Before(ci.expireTime) {
			return ci.value, true
		}
		c.data.Delete(key)
	}
	return nil, false
}

// GetString 获取字符串缓存
func (c *LocalCache) GetString(key string) (string, bool) {
	if v, ok := c.Get(key); ok {
		if s, ok := v.(string); ok {
			return s, true
		}
	}
	return "", false
}

// GetInt64 获取int64缓存
func (c *LocalCache) GetInt64(key string) (int64, bool) {
	if v, ok := c.Get(key); ok {
		if i, ok := v.(int64); ok {
			return i, true
		}
	}
	return 0, false
}

// Set 设置缓存（使用默认TTL）
func (c *LocalCache) Set(key string, value interface{}) {
	c.SetWithTTL(key, value, c.ttl)
}

// SetWithTTL 设置缓存（自定义TTL）
func (c *LocalCache) SetWithTTL(key string, value interface{}, ttl time.Duration) {
	c.data.Store(key, &cacheItem{
		value:      value,
		expireTime: time.Now().Add(ttl),
	})
}

// Delete 删除缓存
func (c *LocalCache) Delete(key string) {
	c.data.Delete(key)
}

// Exists 检查键是否存在
func (c *LocalCache) Exists(key string) bool {
	_, ok := c.Get(key)
	return ok
}

// GetOrSet 获取或设置缓存
func (c *LocalCache) GetOrSet(key string, setter func() (interface{}, error)) (interface{}, error) {
	if v, ok := c.Get(key); ok {
		return v, nil
	}

	v, err, _ := c.group.Do(key, func() (interface{}, error) {
		// 双重检查：在获取到 singleflight 锁后再次检查缓存
		if v, ok := c.Get(key); ok {
			return v, nil
		}
		value, err := setter()
		if err != nil {
			return nil, err
		}
		c.Set(key, value)
		return value, nil
	})
	return v, err
}

// GetOrSetWithTTL 获取或设置缓存（自定义TTL）
func (c *LocalCache) GetOrSetWithTTL(key string, ttl time.Duration, setter func() (interface{}, error)) (interface{}, error) {
	if v, ok := c.Get(key); ok {
		return v, nil
	}

	v, err, _ := c.group.Do(key, func() (interface{}, error) {
		// 双重检查：在获取到 singleflight 锁后再次检查缓存
		if v, ok := c.Get(key); ok {
			return v, nil
		}
		value, err := setter()
		if err != nil {
			return nil, err
		}
		c.SetWithTTL(key, value, ttl)
		return value, nil
	})
	return v, err
}

// Clear 清空所有缓存
func (c *LocalCache) Clear() {
	c.data.Range(func(key, value interface{}) bool {
		c.data.Delete(key)
		return true
	})
}

// Size 获取缓存大小
func (c *LocalCache) Size() int {
	count := 0
	c.data.Range(func(key, value interface{}) bool {
		count++
		return true
	})
	return count
}

// Keys 获取所有键
func (c *LocalCache) Keys() []string {
	keys := make([]string, 0)
	c.data.Range(func(key, value interface{}) bool {
		if k, ok := key.(string); ok {
			keys = append(keys, k)
		}
		return true
	})
	return keys
}

// cleanExpired 清理过期缓存
func (c *LocalCache) cleanExpired() {
	for {
		select {
		case <-c.cleaner.C:
			now := time.Now()
			c.data.Range(func(key, value interface{}) bool {
				if item, ok := value.(*cacheItem); ok {
					if now.After(item.expireTime) {
						c.data.Delete(key)
					}
				}
				return true
			})
		case <-c.stopCh:
			return
		}
	}
}

// Stop 停止清理协程
func (c *LocalCache) Stop() {
	c.stopOnce.Do(func() {
		c.cleaner.Stop()
		close(c.stopCh)
	})
}

// Stats 缓存统计
type Stats struct {
	Size       int
	TTL        time.Duration
	ExpiredCnt int
}

// GetStats 获取缓存统计
func (c *LocalCache) GetStats() Stats {
	size := 0
	expired := 0
	now := time.Now()

	c.data.Range(func(key, value interface{}) bool {
		size++
		if item, ok := value.(*cacheItem); ok {
			if now.After(item.expireTime) {
				expired++
			}
		}
		return true
	})

	return Stats{
		Size:       size,
		TTL:        c.ttl,
		ExpiredCnt: expired,
	}
}
