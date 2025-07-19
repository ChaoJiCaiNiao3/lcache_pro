package store

import "time"

// Value 缓存值接口
type Value interface {
	Len() int // 返回数据大小
}

// Store 缓存接口
type Store interface {
	Get(key string) (Value, bool)
	Set(key string, value Value) error
	SetWithExpiration(key string, value Value, expiration time.Duration) error
	Delete(key string) bool
	Clear()
	Len() int
	Close()
}

// CacheType 缓存类型
type CacheType string

const (
	LRU  CacheType = "lru"
	LRU2 CacheType = "lru2"
)

// Options 通用缓存配置选项
type Options struct {
	MaxBytes        int64  // 最大的缓存字节数（用于 lru）
	BucketCount     uint16 // 缓存的桶数量（用于 lru-2）
	CapPerBucket    uint16 // 每个桶的容量（用于 lru-2）
	Level2Cap       uint16 // lru-2 中二级缓存的容量（用于 lru-2）
	CleanupInterval time.Duration
	OnEvicted       func(key string, value Value)
}

func NewOptions() Options {
	return Options{
		MaxBytes:        8192,
		BucketCount:     16,
		CapPerBucket:    512,
		Level2Cap:       256,
		CleanupInterval: time.Minute,
		OnEvicted:       nil,
	}
}

// NewStore 创建缓存存储实例
func NewStore(cacheType CacheType, opts Options) Store {
	switch cacheType {
	case LRU2:
		return newLRU2Cache(opts)
	case LRU:
		return newLRUCache(opts)
	default:
		return newLRUCache(opts)
	}
}
