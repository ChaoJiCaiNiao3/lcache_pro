package consistenthash

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ChaoJiCaiNiao3/lcache_pro/registry"
	"github.com/sirupsen/logrus"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"
)

// Map 一致性哈希实现
type Map struct {
	mu sync.RWMutex
	// 配置信息
	config *Config
	// 哈希环
	keys []int
	// 哈希环到节点的映射
	hashMap map[int]string
	// 节点到虚拟节点数量的映射
	nodeReplicas map[string]int
	// 节点负载统计
	nodeCounts map[string]int64
	//etcd相关
	selfAddr string
	etcdCli  *clientv3.Client
	ctx      context.Context
	cancel   context.CancelFunc
}

// 哈希环需要传递给其它节点的数据
type HashRing struct {
	Keys    []int
	HashMap map[int]string
}

// New 创建一致性哈希实例
func NewConsistentHash(opts ...Option) *Map {
	ctx, cancel := context.WithCancel(context.Background())
	m := &Map{
		config:       DefaultConfig,
		hashMap:      make(map[int]string),
		nodeReplicas: make(map[string]int),
		nodeCounts:   make(map[string]int64),
		ctx:          ctx,
		cancel:       cancel,
	}

	for _, opt := range opts {
		opt(m)
	}

	etcdCli, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{"localhost:2379"},
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		logrus.Errorf("failed to create etcd client: %v", err)
	}
	m.etcdCli = etcdCli

	// etcdClient来向etcd中传送自己的负载统计
	go func() {
		ticker := time.NewTicker(time.Second * 10)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				registry.PutEtcdConHashNodeCount(m.etcdCli, m.selfAddr, m.nodeCounts[m.selfAddr])
			}
		}
	}()
	// etcdClient发起竞选
	go m.RunElection(ctx)
	return m
}

// 修改 RunElection 支持 context 传递
func (m *Map) RunElection(ctx context.Context) error {
	for {
		session, err := concurrency.NewSession(m.etcdCli)
		if err != nil {
			return err
		}
		election := concurrency.NewElection(session, "/consistenthash/leader")
		cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		//竞选
		err = election.Campaign(cctx, m.selfAddr)
		if err == nil {
			//成为领导了，进行nodeCount的更新和哈希环同步
			go m.updateNodeCount(ctx)
			go m.CheckAndRebalanceAndSyncHashRing(ctx)
			cancel()
			return nil
		} else {
			//竞选失败,监听领导者的变化并同步哈希环
			ch := election.Observe(ctx)
			go m.updateHashRing(ctx)
			for {
				select {
				case <-ctx.Done():
					cancel()
					return ctx.Err()
				case resp, ok := <-ch:
					if !ok {
						time.Sleep(time.Second * 2)
						break
					}
					fmt.Println("新 leader:", string(resp.Kvs[0].Value))
				}
			}
		}
	}
}

// Option 配置选项
type Option func(*Map)

func (m *Map) updateNodeCount(ctx context.Context) error {
	//先进行全量更新
	m.fetchAllNodeCount(ctx)

	//启动增量更新
	go m.watchNodeCount(ctx)
	return nil
}

func (m *Map) fetchAllNodeCount(ctx context.Context) error {
	resp, err := m.etcdCli.Get(ctx, "/conhashNodeCount/", clientv3.WithPrefix())
	if err != nil {
		return err
	}

	for _, kv := range resp.Kvs {
		addr := strings.TrimPrefix(string(kv.Key), "/conhashNodeCount/")
		count, err := strconv.ParseInt(string(kv.Value), 10, 64)
		if err != nil {
			return err
		}
		m.nodeCounts[addr] = count
	}
	return nil
}

func (m *Map) watchNodeCount(ctx context.Context) error {
	watchCh := m.etcdCli.Watch(ctx, "/conhashNodeCount/", clientv3.WithPrefix())
	for resp := range watchCh {
		for _, event := range resp.Events {
			addr := strings.TrimPrefix(string(event.Kv.Key), "/conhashNodeCount/")
			count, err := strconv.ParseInt(string(event.Kv.Value), 10, 64)
			if err != nil {
				return err
			}
			m.nodeCounts[addr] = count
		}
	}
	return nil
}

func (m *Map) CheckAndRebalanceAndSyncHashRing(ctx context.Context) error {
	//计算时间，每10s进行一次负载均衡(使用time.Ticker)
	ticker := time.NewTicker(time.Second * 10)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			err, IsRebalance := m.checkAndRebalance()
			if err != nil {
				logrus.Errorf("failed to check and rebalance: %v", err)
			}
			if IsRebalance {
				m.syncHashRing()
			}
		}
	}
}

func (m *Map) updateHashRing(ctx context.Context) error {
	watchCh := m.etcdCli.Watch(ctx, "/consistenthash/hashring", clientv3.WithPrefix())
	for resp := range watchCh {
		for _, event := range resp.Events {
			hashRing := HashRing{}
			json.Unmarshal(event.Kv.Value, &hashRing)
			m.mu.Lock()
			m.keys = hashRing.Keys
			m.hashMap = hashRing.HashMap
			m.nodeCounts[m.selfAddr] = 0
			m.mu.Unlock()
		}
	}
	return nil
}

func (m *Map) syncHashRing() error {
	hashRing := HashRing{
		Keys:    m.keys,
		HashMap: m.hashMap,
	}
	hashRingData, _ := json.Marshal(hashRing)
	m.etcdCli.Put(context.Background(), "/consistenthash/hashring", string(hashRingData))
	//把所有的nodeCount都设置为0
	for addr := range m.nodeCounts {
		m.nodeCounts[addr] = 0
		//同步到etcd
		registry.PutEtcdConHashNodeCount(m.etcdCli, addr, 0)
	}

	return nil
}

func (m *Map) Add(addr string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.nodeReplicas[addr]; ok {
		return nil
	}

	replicas := m.config.DefaultReplicas
	err := m.addNode(addr, replicas)
	if err != nil {
		return err
	}
	return nil
}

func (m *Map) addNode(addr string, replicas int) error {
	for i := 0; i < replicas; i++ {
		hash := int(m.config.HashFunc([]byte(fmt.Sprintf("%s-%d", addr, i))))
		m.keys = append(m.keys, hash)
		m.hashMap[hash] = addr
		m.nodeCounts[addr] = 0
	}
	m.nodeReplicas[addr] = replicas

	sort.Ints(m.keys)
	return nil
}

func (m *Map) Remove(addr string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.nodeReplicas[addr]; !ok {
		return nil
	}

	err := m.removeNoLock(addr)
	if err != nil {
		return err
	}
	return nil
}

func (m *Map) removeNoLock(addr string) error {
	replicas := m.nodeReplicas[addr]
	for i := 0; i < replicas; i++ {
		hash := int(m.config.HashFunc([]byte(fmt.Sprintf("%s-%d", addr, i))))
		delete(m.hashMap, hash)
		m.keys = slice_remove(m.keys, hash)
	}
	delete(m.nodeReplicas, addr)
	delete(m.nodeCounts, addr)
	return nil
}

func slice_remove(slice []int, value int) []int {
	//二分查找排好序的切片的值
	index := sort.SearchInts(slice, value)
	if index < len(slice) && slice[index] == value {
		return append(slice[:index], slice[index+1:]...)
	}
	return slice
}

func (m *Map) Get(key string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if len(m.keys) == 0 {
		return ""
	}

	hash := int(m.config.HashFunc([]byte(key)))
	index := sort.Search(len(m.keys), func(i int) bool {
		return m.keys[i] >= hash
	})
	if index == len(m.keys) {
		index = 0
	}

	return m.hashMap[m.keys[index]]
}

func (m *Map) checkAndRebalance() (error, bool) {
	total := int64(0)
	for _, val := range m.nodeCounts {
		total += val
	}

	avg := total / int64(len(m.nodeCounts))

	IsRebalance := false
	for addr, val := range m.nodeCounts {
		newReplicas := m.nodeReplicas[m.selfAddr]
		flag := false
		if float64(val)/float64(avg) > m.config.MaxLoadBalanceThreshold {
			newReplicas = m.nodeReplicas[addr] - 10
			flag = true
		} else if float64(val)/float64(avg) < m.config.MinLoadBalanceThreshold {
			newReplicas = m.nodeReplicas[addr] + 10
			flag = true
		}
		if newReplicas > m.config.MaxReplicas {
			newReplicas = m.config.MaxReplicas
		}
		if newReplicas < m.config.MinReplicas {
			newReplicas = m.config.MinReplicas
		}
		if flag {
			IsRebalance = true
			err := m.rebalanceReplicas(addr, newReplicas)
			if err != nil {
				logrus.Errorf("failed to rebalance replicas for %s: %v", addr, err)
				return err, IsRebalance
			}
		}
	}
	return nil, IsRebalance
}

func (m *Map) rebalanceReplicas(addr string, newReplicas int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	err := m.removeNoLock(addr)
	if err != nil {
		return err
	}
	err = m.addNode(addr, newReplicas)
	if err != nil {
		return err
	}
	return nil
}

func (m *Map) Close() error {
	if m.cancel != nil {
		m.cancel()
	}
	if m.etcdCli != nil {
		m.etcdCli.Close()
	}
	return nil
}
