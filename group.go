package lcache_pro

import (
	"github.com/ChaoJiCaiNiao3/lcache_pro/store"
)

type Group struct {
	svcName  string
	selfAddr string
	cache    store.Store
	opts     store.Options
}

func NewGroup(svcName string, selfAddr string, opts store.Options, cacheType store.CacheType) *Group {
	cache := store.NewStore(cacheType, opts)
	return &Group{
		svcName:  svcName,
		selfAddr: selfAddr,
		cache:    cache,
		opts:     opts,
	}
}

func (g *Group) Close() error {
	g.cache.Close()
	return nil
}
