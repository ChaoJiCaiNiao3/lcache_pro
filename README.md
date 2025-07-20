# 🚀 lcache_pro - 基于 KamaCache 重构改良的分布式缓存系统

> 高效 · 一致性哈希 · etcd 管理 · LRU/LRU2 · gRPC · Redis

---

## 📚 目录
- [项目概述](#项目概述)
- [代码结构](#代码结构)
- [安装和使用](#安装和使用)
- [配置说明](#配置说明)
- [注意事项](#注意事项)
- [贡献和反馈](#贡献和反馈)
- [许可证](#许可证)

---

## 📝 项目概述

**lcache_pro** 是一个基于 KamaCache 进行重构的缓存系统，实现了真正的一致性哈希算法，确保各个节点的哈希环保持一致。该系统支持分布式缓存，通过 etcd 进行节点管理和负载均衡，提供高效、稳定的缓存服务。

---

## 🗂️ 代码结构

| 目录/文件                  | 说明                                   |
|----------------------------|----------------------------------------|
| `consistenthash/`          | 一致性哈希算法实现（含配置与核心逻辑） |
| ├─ `config.go`             | 一致性哈希配置信息                     |
| └─ `con_hash.go`           | 哈希环管理、节点增删、负载均衡等        |
| `peers.go`                 | 缓存节点接口定义                       |
| `registry/`                | etcd 交互工具函数，节点负载统计         |
| `cacheTest/`               | 测试代码，服务启动与缓存操作示例        |
| `store/`                   | 缓存存储实现，含 LRU/LRU2 算法         |
| ├─ `lru.go`                | LRU 缓存实现                           |
| ├─ `lru2.go`               | LRU2 缓存实现                          |
| └─ `lru2_test.go`          | LRU2 单元测试                          |
| `pb/`                      | gRPC 相关代码                          |
| `server.go`                | 服务器端缓存操作接口                   |
| `database/`                | 数据库交互实现（目前支持 Redis）        |
| └─ `redis.go`              | Redis 操作接口                         |

---

## ⚡ 安装和使用

### 1. 安装依赖
确保你已经安装了 Go 语言环境，并使用以下命令下载项目依赖：
```bash
go mod tidy
```

### 2. 启动服务
先启动中间件（etcd、redis）：
```bash
cd DockerService
docker-compose up -d
```

再启动 main.go 进行测试：
```bash
go run cacheTest/main.go
```

### 3. 交互命令
| 命令      | 说明                                 |
|-----------|--------------------------------------|
| `get`     | 从缓存和 Redis 获取指定 key 的值      |
| `set`     | 将 key-value 存入缓存和 Redis         |
| `delete`  | 从缓存和 Redis 删除指定 key           |
| `set_hot` | 只存入本地缓存                       |
| `exit`    | 关闭服务                             |

---

## ⚙️ 配置说明

### 一致性哈希配置
在 `consistenthash/config.go` 文件中，可配置如下参数：
- `DefaultReplicas`：每个真实节点的默认虚拟节点数
- `MinReplicas`：最小虚拟节点数
- `MaxReplicas`：最大虚拟节点数
- `HashFunc`：哈希函数，默认 `crc32.ChecksumIEEE`
- `MaxLoadBalanceThreshold`：负载均衡最大阈值，超过则调整虚拟节点
- `MinLoadBalanceThreshold`：负载均衡最小阈值，低于则调整虚拟节点

### Redis 配置
在 `database/redis.go` 文件中，可配置 Redis 连接信息：
- `RedisHost`：Redis 服务器主机名
- `RedisPort`：Redis 服务器端口号
- `RedisPassword`：Redis 密码
- `RedisDB`：使用的 Redis 数据库编号

---

## ⚠️ 注意事项
- 确保 etcd 服务已启动，且 `consistenthash/con_hash.go` 中 etcd 连接信息（Endpoints）正确。
- 缓存操作时注意处理网络异常、Redis 连接失败等错误。

---

## 🙏 新手声明

> 本项目由个人学习和兴趣驱动开发，作者仍在不断学习和成长，项目中难免存在设计不完善、代码不规范或功能不全等问题。欢迎大家批评指正、提出建议或参与改进，非常感谢您的关注与支持！

---

## 🤝 贡献和反馈
如有建议或发现问题，欢迎在 GitHub 仓库提交 issue 或 pull request。

---

## 📄 许可证
本项目采用 MIT 许可证，详见 [LICENSE](./LICENSE) 文件。

---
