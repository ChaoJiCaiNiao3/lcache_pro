lcache_pro - 基于 KamaCache 重构改良的缓存系统
项目概述
lcache_pro 是一个基于 KamaCache 进行重构改良的缓存系统，实现了真正的一致性哈希算法，确保各个节点的哈希环保持一致。该系统支持分布式缓存，通过 etcd 进行节点管理和负载均衡，提供了高效、稳定的缓存服务。

代码结构
主要文件和目录
lcache_pro/consistenthash：一致性哈希算法的实现，包括配置文件和核心逻辑。
config.go：一致性哈希的配置信息。
con_hash.go：一致性哈希的核心实现，包括哈希环的管理、节点的添加和删除、负载均衡等功能。
lcache_pro/peers.go：定义了缓存节点的接口。
lcache_pro/registry：与 etcd 交互的工具函数，用于节点负载统计的存储和读取。
lcache_pro/cacheTest：测试代码，包含启动服务和缓存操作的示例。
lcache_pro/store：缓存存储的实现，包括 LRU 和 LRU2 缓存算法。
lru.go：LRU 缓存的实现。
lru2.go：LRU2 缓存的实现。
lru2_test.go：LRU2 缓存的单元测试。
lcache_pro/pb：grpc相关代码。
lcache_pro/server.go：服务器端的缓存操作接口。
lcache_pro/database：数据库交互的实现，目前仅支持 Redis。
lcache_pro/database/redis.go：Redis 数据库的操作接口。

安装和使用

安装依赖
确保你已经安装了 Go 语言环境，并使用以下命令下载项目依赖：
go mod tidy
启动服务
先启动中间件
docker-compose up -d
//启动main.go测试
go run lcache_pro/cacheTest/main.go
启动服务后，你可以通过命令行输入以下操作：
get：从缓存和 Redis 中获取指定 key 的值。
set：将指定 key-value 对存入缓存和 Redis。
delete：从缓存和 Redis 中删除指定 key 的值。
set_hot：将指定 key-value 对存入缓存。
exit：关闭服务。
配置说明
一致性哈希配置
在 lcache_pro/consistenthash/config.go 文件中，可以配置以下参数：
DefaultReplicas：每个真实节点对应的默认虚拟节点数。
MinReplicas：最小虚拟节点数。
MaxReplicas：最大虚拟节点数。
HashFunc：哈希函数，默认使用 crc32.ChecksumIEEE。
MaxLoadBalanceThreshold：负载均衡的最大阈值，超过此值触发虚拟节点调整。
MinLoadBalanceThreshold：负载均衡的最小阈值，低于此值触发虚拟节点调整。
Redis 配置
在 lcache_pro/database/redis.go 文件中，可以配置 Redis 的连接信息：
RedisHost：Redis 服务器的主机名。
RedisPort：Redis 服务器的端口号。
RedisPassword：Redis 服务器的密码。
RedisDB：使用的 Redis 数据库编号。
注意事项
确保 etcd 服务已经启动，并且 lcache_pro/consistenthash/con_hash.go 文件中的 etcd 连接信息（Endpoints）正确。
在进行缓存操作时，注意处理可能出现的错误，如网络异常、Redis 连接失败等。
贡献和反馈
如果你对本项目有任何建议或发现了问题，请在项目的 GitHub 仓库中提交 issue 或 pull request。
许可证
本项目采用 MIT 许可证，详细信息请参考 lcache_pro/LICENSE 文件。