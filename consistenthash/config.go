package consistenthash

import "hash/crc32"

// Config 一致性哈希配置
type Config struct {
	// 每个真实节点对应的虚拟节点数
	DefaultReplicas int
	// 最小虚拟节点数
	MinReplicas int
	// 最大虚拟节点数
	MaxReplicas int
	// 哈希函数
	HashFunc func(data []byte) uint32
	// 负载均衡阈值，超过此值触发虚拟节点调整
	MaxLoadBalanceThreshold float64
	MinLoadBalanceThreshold float64
}

// DefaultConfig 默认配置
var DefaultConfig = &Config{
	DefaultReplicas:         50,
	MinReplicas:             10,
	MaxReplicas:             200,
	HashFunc:                crc32.ChecksumIEEE,
	MaxLoadBalanceThreshold: 1.1,
	MinLoadBalanceThreshold: 0.9,
}
