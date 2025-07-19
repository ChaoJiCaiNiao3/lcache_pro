package store

import (
	"fmt"
	"strconv"
	"sync"
	"testing"
	"time"
)

// 为测试定义一个简单的Value类型
type testValue string

func (v testValue) Len() int {
	return len(v)
}

// 测试缓存基本操作
func TestCacheBasic(t *testing.T) {
	t.Run("初始化缓存", func(t *testing.T) {
		c := Create(10)
		if c == nil {
			t.Fatal("创建缓存失败")
		}
		if c.last != 0 {
			t.Fatalf("初始last应为0，实际为%d", c.last)
		}
		if len(c.m) != 10 {
			t.Fatalf("缓存容量应为10，实际为%d", len(c.m))
		}
		if len(c.dlnk) != 11 {
			t.Fatalf("链表长度应为cap+1(11)，实际为%d", len(c.dlnk))
		}
	})

	t.Run("添加和获取", func(t *testing.T) {
		c := Create(5)
		var evictCount int
		onEvicted := func(key string, value Value) {
			evictCount++
		}

		// 添加新项
		status := c.put("key1", testValue("value1"), 100, onEvicted)
		if status != 1 {
			t.Fatalf("添加新项应返回1，实际返回%d", status)
		}
		if c.last != 1 {
			t.Fatalf("添加一项后last应为1，实际为%d", c.last)
		}

		// 获取项
		node, status := c.get("key1")
		if status != 1 {
			t.Fatalf("获取存在项应返回1，实际返回%d", status)
		}
		if node == nil {
			t.Fatal("获取项返回了nil")
		}
		if node.k != "key1" || node.v.(testValue) != "value1" || node.expireAt != 100 {
			t.Fatalf("获取项值不一致: %+v", *node)
		}

		// 获取不存在的项
		node, status = c.get("不存在")
		if status != 0 {
			t.Fatalf("获取不存在项应返回0，实际返回%d", status)
		}
		if node != nil {
			t.Fatal("获取不存在项不应返回节点")
		}

		// 更新现有项
		status = c.put("key1", testValue("新值"), 200, onEvicted)
		if status != 0 {
			t.Fatalf("更新项应返回0，实际返回%d", status)
		}

		// 验证更新后的值
		node, _ = c.get("key1")
		if node.v.(testValue) != "新值" || node.expireAt != 200 {
			t.Fatalf("更新项后值不一致: %+v", *node)
		}
	})

	t.Run("删除操作", func(t *testing.T) {
		c := Create(5)

		// 添加项
		c.put("key1", testValue("value1"), 100, nil)

		// 删除存在的项
		node, status, expireAt := c.del("key1")
		if status != 1 {
			t.Fatalf("删除存在项应返回1，实际返回%d", status)
		}
		if node == nil {
			t.Fatal("删除应返回被删除的节点")
		}
		if node.expireAt != 0 {
			t.Fatalf("删除后节点expireAt应为0，实际为%d", node.expireAt)
		}
		if expireAt != 100 {
			t.Fatalf("删除应返回原始expireAt(100)，实际为%d", expireAt)
		}

		// 验证删除后无法获取
		node, status = c.get("key1")
		if status != 1 {
			t.Fatal("获取已删除项失败，但键仍应存在于哈希表中")
		}
		if node.expireAt != 0 {
			t.Fatalf("已删除项的expireAt应为0，实际为%d", node.expireAt)
		}

		// 删除不存在的项
		node, status, _ = c.del("不存在")
		if status != 0 {
			t.Fatalf("删除不存在项应返回0，实际返回%d", status)
		}
		if node != nil {
			t.Fatal("删除不存在项不应返回节点")
		}
	})

	t.Run("容量和淘汰", func(t *testing.T) {
		c := Create(3) // 容量为3的缓存
		var evictedKeys []string

		onEvicted := func(key string, value Value) {
			evictedKeys = append(evictedKeys, key)
		}

		// 填满缓存
		for i := 1; i <= 3; i++ {
			c.put("key"+string(rune('0'+i)), testValue("value"+string(rune('0'+i))), 100, onEvicted)
		}

		// 再添加一项，应该淘汰最早的key1
		c.put("key4", testValue("value4"), 100, onEvicted)

		if len(evictedKeys) != 1 {
			t.Fatalf("应淘汰1项，实际淘汰%d项", len(evictedKeys))
		}
		if evictedKeys[0] != "key1" {
			t.Fatalf("应淘汰key1，实际淘汰%s", evictedKeys[0])
		}

		// 验证缓存状态
		_, status := c.get("key1")
		if status != 0 {
			t.Fatal("key1应已被淘汰")
		}

		for i := 2; i <= 4; i++ {
			node, status := c.get("key" + string(rune('0'+i)))
			if status != 1 || node == nil {
				t.Fatalf("key%d应存在于缓存中", i)
			}
		}
	})

	t.Run("LRU顺序维护", func(t *testing.T) {
		c := Create(3)

		// 按顺序添加3项
		for i := 1; i <= 3; i++ {
			c.put("key"+string(rune('0'+i)), testValue("value"+string(rune('0'+i))), 100, nil)
		}

		// 访问顺序：key1 (最后访问)，key2, key3 (最早访问)
		c.get("key2")
		c.get("key1")

		// 添加新项，应淘汰key3
		c.put("key4", testValue("value4"), 100, nil)

		// 验证key3被淘汰
		node, status := c.get("key3")
		if status != 0 || node != nil {
			t.Fatal("key3应已被淘汰")
		}

		// 其他键应该存在
		for i := 1; i <= 4; i++ {
			if i == 3 {
				continue
			}
			_, status := c.get("key" + string(rune('0'+i)))
			if status != 1 {
				t.Fatalf("key%d应存在于缓存中", i)
			}
		}
	})

	t.Run("遍历缓存", func(t *testing.T) {
		c := Create(5)

		// 添加3项
		for i := 1; i <= 3; i++ {
			c.put("key"+string(rune('0'+i)), testValue("value"+string(rune('0'+i))), 100, nil)
		}

		// 遍历并收集所有键
		var keys []string
		c.walk(func(key string, value Value, expireAt int64) bool {
			keys = append(keys, key)
			return true
		})

		// 应有3个键
		if len(keys) != 3 {
			t.Fatalf("应有3个键，实际有%d个", len(keys))
		}

		// 键应该是反向添加顺序（因为新项是添加到链表头）
		expectedKeys := []string{"key3", "key2", "key1"}
		for i, key := range expectedKeys {
			if i >= len(keys) || keys[i] != key {
				t.Fatalf("第%d个键应为%s，实际为%s", i, key, keys[i])
			}
		}

		// 测试提前终止遍历
		var earlyKeys []string
		c.walk(func(key string, value Value, expireAt int64) bool {
			earlyKeys = append(earlyKeys, key)
			return len(earlyKeys) < 2 // 只收集前2个键
		})

		// 应只有2个键
		if len(earlyKeys) != 2 {
			t.Fatalf("应有2个键，实际有%d个", len(earlyKeys))
		}
	})
}

// 测试缓存容量限制和LRU替换策略
func TestCacheLRUEviction(t *testing.T) {
	var evictedKeys []string
	onEvicted := func(key string, value Value) {
		evictedKeys = append(evictedKeys, key)
	}

	// 创建一个容量为3的缓存
	c := Create(3)

	// 添加3个项，不应该有淘汰
	c.put("key1", testValue("value1"), Now()+int64(time.Hour), onEvicted)
	c.put("key2", testValue("value2"), Now()+int64(time.Hour), onEvicted)
	c.put("key3", testValue("value3"), Now()+int64(time.Hour), onEvicted)

	if len(evictedKeys) != 0 {
		t.Errorf("Expected no evictions, got %v", evictedKeys)
	}

	// 访问key1使其成为最近使用的
	c.get("key1")

	// 添加第4个项，应该淘汰最少使用的key2
	c.put("key4", testValue("value4"), Now()+int64(time.Hour), onEvicted)

	if len(evictedKeys) != 1 || evictedKeys[0] != "key2" {
		t.Errorf("Expected key2 to be evicted, got %v", evictedKeys)
	}

	// 验证key2已被淘汰
	node, status := c.get("key2")
	if status != 0 || node != nil {
		t.Errorf("Expected key2 to be evicted")
	}

	// 验证其他键仍然存在
	keys := []string{"key1", "key3", "key4"}
	for _, key := range keys {
		node, status := c.get(key)
		if status != 1 || node == nil {
			t.Errorf("Expected %s to exist", key)
		}
	}
}

// 测试walk方法
func TestCacheWalk(t *testing.T) {
	c := Create(5)

	// 添加几个项
	c.put("key1", testValue("value1"), Now()+int64(time.Hour), nil)
	c.put("key2", testValue("value2"), Now()+int64(time.Hour), nil)
	c.put("key3", testValue("value3"), Now()+int64(time.Hour), nil)

	// 删除一个项
	c.del("key2")

	// 使用walk收集所有项
	var keys []string
	c.walk(func(key string, value Value, expireAt int64) bool {
		keys = append(keys, key)
		return true
	})

	// 验证只有未删除的项被遍历
	if len(keys) != 2 || !contains(keys, "key1") || !contains(keys, "key3") || contains(keys, "key2") {
		t.Errorf("Walk didn't return expected keys, got %v", keys)
	}

	// 测试提前终止遍历
	count := 0
	c.walk(func(key string, value Value, expireAt int64) bool {
		count++
		return false // 只处理第一个项
	})

	if count != 1 {
		t.Errorf("Walk didn't stop early as expected")
	}
}

// 测试adjust方法
func TestCacheAdjust(t *testing.T) {
	c := Create(5)

	// 添加几个项以形成链表
	c.put("key1", testValue("value1"), Now()+int64(time.Hour), nil)
	c.put("key2", testValue("value2"), Now()+int64(time.Hour), nil)
	c.put("key3", testValue("value3"), Now()+int64(time.Hour), nil)

	// 获取key1的索引
	idx1 := c.hmap["key1"]

	// 将key1移动到链表头部
	c.adjust(idx1, p, n)

	// 验证key1现在是最近使用的
	if c.dlnk[0][n] != idx1 {
		t.Errorf("Expected key1 to be at the head of the list")
	}

	// 将key1移动到链表尾部
	c.adjust(idx1, n, p)

	// 验证key1现在是最少使用的
	if c.dlnk[0][p] != idx1 {
		t.Errorf("Expected key1 to be at the tail of the list")
	}
}

// 测试lru2Store的基本接口
func TestLRU2StoreBasicOperations(t *testing.T) {
	var evictedKeys []string
	onEvicted := func(key string, value Value) {
		evictedKeys = append(evictedKeys, fmt.Sprintf("%s:%v", key, value))
	}

	opts := Options{
		BucketCount:     4,
		CapPerBucket:    2,
		Level2Cap:       3,
		CleanupInterval: time.Minute,
		OnEvicted:       onEvicted,
	}

	store := newLRU2Cache(opts)
	defer store.Close()

	// 测试Set和Get
	err := store.Set("key1", testValue("value1"))
	if err != nil {
		t.Errorf("Set failed: %v", err)
	}

	value, found := store.Get("key1")
	if !found || value != testValue("value1") {
		t.Errorf("Get failed, expected 'value1', got %v, found: %v", value, found)
	}

	// 测试更新
	err = store.Set("key1", testValue("value1-updated"))
	if err != nil {
		t.Errorf("Set update failed: %v", err)
	}

	value, found = store.Get("key1")
	if !found || value != testValue("value1-updated") {
		t.Errorf("Get after update failed, expected 'value1-updated', got %v", value)
	}

	// 测试不存在的键
	value, found = store.Get("nonexistent")
	if found {
		t.Errorf("Get nonexistent key should return false, got %v, %v", value, found)
	}

	// 测试删除
	deleted := store.Delete("key1")
	if !deleted {
		t.Errorf("Delete should return true")
	}

	value, found = store.Get("key1")
	if found {
		t.Errorf("Get after delete should return false, got %v, %v", value, found)
	}

	// 测试删除不存在的键
	deleted = store.Delete("nonexistent")
	if deleted {
		t.Errorf("Delete nonexistent key should return false")
	}
}

// 测试LRU2Store的LRU替换策略
func TestLRU2StoreLRUEviction(t *testing.T) {
	var evictedKeys []string
	onEvicted := func(key string, value Value) {
		evictedKeys = append(evictedKeys, key)
	}

	opts := Options{
		BucketCount:     1, // 单桶以简化测试
		CapPerBucket:    2, // 一级缓存容量
		Level2Cap:       2, // 二级缓存容量
		CleanupInterval: time.Minute,
		OnEvicted:       onEvicted,
	}

	store := newLRU2Cache(opts)
	defer store.Close()

	// 添加超过一级缓存容量的项
	store.Set("key1", testValue("value1"))
	store.Set("key2", testValue("value2"))
	store.Set("key3", testValue("value3")) // 应该淘汰key1到二级缓存

	// key1应该在二级缓存中
	value, found := store.Get("key1")
	if !found || value != testValue("value1") {
		t.Errorf("key1 should be in level2 cache, got %v, found: %v", value, found)
	}

	// 添加更多项，超过二级缓存容量
	store.Set("key4", testValue("value4")) // 应该淘汰key2到二级缓存
	store.Set("key5", testValue("value5")) // 应该淘汰key3，key1应该从二级缓存中被淘汰

	// key1应该已被完全淘汰
	value, found = store.Get("key1")
	if found {
		t.Errorf("key1 should be evicted, got %v, found: %v", value, found)
	}
}

// 测试过期时间
func TestLRU2StoreExpiration(t *testing.T) {
	opts := Options{
		BucketCount:     1,
		CapPerBucket:    5,
		Level2Cap:       5,
		CleanupInterval: 100 * time.Millisecond, // 快速清理
		OnEvicted:       nil,
	}

	store := newLRU2Cache(opts)
	defer store.Close()

	// 添加一个很快过期的项
	shortDuration := 200 * time.Millisecond
	store.SetWithExpiration("expires-soon", testValue("value"), shortDuration)

	// 添加一个不会很快过期的项
	store.SetWithExpiration("expires-later", testValue("value"), time.Hour)

	// 验证都能获取到
	_, found := store.Get("expires-soon")
	if !found {
		t.Errorf("expires-soon should be found initially")
	}

	_, found = store.Get("expires-later")
	if !found {
		t.Errorf("expires-later should be found")
	}

	// 等待短期项过期
	time.Sleep(300 * time.Millisecond)

	// 验证短期项已过期，长期项仍存在
	_, found = store.Get("expires-soon")
	if found {
		t.Errorf("expires-soon should have expired")
	}

	_, found = store.Get("expires-later")
	if !found {
		t.Errorf("expires-later should still be valid")
	}
}

// 测试LRU2Store的清理循环
func TestLRU2StoreCleanupLoop(t *testing.T) {
	opts := Options{
		BucketCount:     1,
		CapPerBucket:    5,
		Level2Cap:       5,
		CleanupInterval: 100 * time.Millisecond, // 快速清理
		OnEvicted:       nil,
	}

	store := newLRU2Cache(opts)
	defer store.Close()

	// 添加几个很快过期的项
	shortDuration := 200 * time.Millisecond
	store.SetWithExpiration("expires1", testValue("value1"), shortDuration)
	store.SetWithExpiration("expires2", testValue("value2"), shortDuration)

	// 添加一个不会很快过期的项
	store.SetWithExpiration("keeps", testValue("value"), time.Hour)

	// 等待项过期并被清理循环处理
	time.Sleep(500 * time.Millisecond)

	// 验证过期项已被清理
	_, found := store.Get("expires1")
	if found {
		t.Errorf("expires1 should have been cleaned up")
	}

	_, found = store.Get("expires2")
	if found {
		t.Errorf("expires2 should have been cleaned up")
	}

	// 验证未过期项仍然存在
	_, found = store.Get("keeps")
	if !found {
		t.Errorf("keeps should still be valid")
	}
}

// 测试LRU2Store的Clear方法
func TestLRU2StoreClear(t *testing.T) {
	opts := Options{
		BucketCount:     2,
		CapPerBucket:    5,
		Level2Cap:       5,
		CleanupInterval: time.Minute,
		OnEvicted:       nil,
	}

	store := newLRU2Cache(opts)
	defer store.Close()

	// 添加一些项
	for i := 0; i < 10; i++ {
		store.Set(fmt.Sprintf("key%d", i), testValue(fmt.Sprintf("value%d", i)))
	}

	// 验证长度
	if length := store.Len(); length != 10 {
		t.Errorf("Expected length 10, got %d", length)
	}

	// 清空缓存
	store.Clear()

	// 验证长度为0
	if length := store.Len(); length != 0 {
		t.Errorf("Expected length 0 after Clear, got %d", length)
	}

	// 验证项已被删除
	for i := 0; i < 10; i++ {
		_, found := store.Get(fmt.Sprintf("key%d", i))
		if found {
			t.Errorf("key%d should not be found after Clear", i)
		}
	}
}

// 测试_get内部方法
func TestLRU2Store_Get(t *testing.T) {
	opts := Options{
		BucketCount:     1,
		CapPerBucket:    5,
		Level2Cap:       5,
		CleanupInterval: time.Minute,
		OnEvicted:       nil,
	}

	store := newLRU2Cache(opts)
	defer store.Close()

	// 向一级缓存添加一个项
	idx := hashBKRD("test-key") & store.mask
	store.caches[idx][0].put("test-key", testValue("test-value"), Now()+int64(time.Hour), nil)

	// 使用_get直接从一级缓存获取
	node, status := store._get("test-key", idx, 0)
	if status != 1 || node == nil || node.v != testValue("test-value") {
		t.Errorf("_get failed to retrieve from level 0")
	}

	// 向二级缓存添加一个项
	store.caches[idx][1].put("test-key2", testValue("test-value2"), Now()+int64(time.Hour), nil)

	// 使用_get直接从二级缓存获取
	node, status = store._get("test-key2", idx, 1)
	if status != 1 || node == nil || node.v != testValue("test-value2") {
		t.Errorf("_get failed to retrieve from level 1")
	}

	// 测试获取不存在的键
	node, status = store._get("nonexistent", idx, 0)
	if status != 0 || node != nil {
		t.Errorf("_get should return status 0 for nonexistent key")
	}

	// 测试过期项
	store.caches[idx][0].put("expired", testValue("value"), Now()-1000, nil) // 已过期
	node, status = store._get("expired", idx, 0)
	if status != 0 || node != nil {
		t.Errorf("_get should return status 0 for expired key")
	}
}

// 测试delete内部方法
func TestLRU2StoreDelete(t *testing.T) {
	var evictedKeys []string
	onEvicted := func(key string, value Value) {
		evictedKeys = append(evictedKeys, key)
	}

	opts := Options{
		BucketCount:     1,
		CapPerBucket:    5,
		Level2Cap:       5,
		CleanupInterval: time.Minute,
		OnEvicted:       onEvicted,
	}

	store := newLRU2Cache(opts)
	defer store.Close()

	// 向一级缓存添加一个项
	idx := hashBKRD("test-key") & store.mask
	store.caches[idx][0].put("test-key", testValue("test-value"), Now()+int64(time.Hour), nil)

	// 向二级缓存添加一个项
	store.caches[idx][1].put("test-key2", testValue("test-value2"), Now()+int64(time.Hour), nil)

	// 删除一级缓存中的项
	deleted := store.delete("test-key", idx)
	if !deleted {
		t.Errorf("delete should return true for existing key")
	}

	// 验证项已被删除且回调被调用
	if len(evictedKeys) != 1 || evictedKeys[0] != "test-key" {
		t.Errorf("OnEvicted callback not called correctly, got %v", evictedKeys)
	}

	// 重置回调记录
	evictedKeys = nil

	// 删除二级缓存中的项
	deleted = store.delete("test-key2", idx)
	if !deleted {
		t.Errorf("delete should return true for existing key in level 1")
	}

	// 验证项已被删除且回调被调用
	if len(evictedKeys) != 1 || evictedKeys[0] != "test-key2" {
		t.Errorf("OnEvicted callback not called correctly, got %v", evictedKeys)
	}

	// 测试删除不存在的键
	deleted = store.delete("nonexistent", idx)
	if deleted {
		t.Errorf("delete should return false for nonexistent key")
	}
}

// 测试并发操作
func TestLRU2StoreConcurrent(t *testing.T) {
	opts := Options{
		BucketCount:     8,
		CapPerBucket:    100,
		Level2Cap:       200,
		CleanupInterval: time.Minute,
		OnEvicted:       nil,
	}

	store := newLRU2Cache(opts)
	defer store.Close()

	const goroutines = 10
	const operationsPerGoroutine = 100

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for g := 0; g < goroutines; g++ {
		go func(id int) {
			defer wg.Done()

			// 每个协程操作自己的一组键
			prefix := fmt.Sprintf("g%d-", id)

			// 添加操作
			for i := 0; i < operationsPerGoroutine; i++ {
				key := prefix + strconv.Itoa(i)
				value := testValue(fmt.Sprintf("value-%s", key))

				err := store.Set(key, value)
				if err != nil {
					t.Errorf("Set failed: %v", err)
				}
			}

			// 获取操作
			for i := 0; i < operationsPerGoroutine; i++ {
				key := prefix + strconv.Itoa(i)
				expectedValue := testValue(fmt.Sprintf("value-%s", key))

				value, found := store.Get(key)
				if !found {
					t.Errorf("Get failed for key %s", key)
				} else if value != expectedValue {
					t.Errorf("Get returned wrong value for %s: expected %s, got %v", key, expectedValue, value)
				}
			}

			// 删除操作
			for i := 0; i < operationsPerGoroutine/2; i++ { // 删除一半的键
				key := prefix + strconv.Itoa(i)
				deleted := store.Delete(key)
				if !deleted {
					t.Errorf("Delete failed for key %s", key)
				}
			}
		}(g)
	}

	wg.Wait()

	// 验证大致长度
	// 每个协程添加了operationsPerGoroutine项，又删除了一半
	expectedItems := goroutines * operationsPerGoroutine / 2
	actualItems := store.Len()

	// 允许一些误差，因为可能有一些键碰撞或未完成的操作
	tolerance := expectedItems / 10
	if actualItems < expectedItems-tolerance || actualItems > expectedItems+tolerance {
		t.Errorf("Expected approximately %d items, got %d", expectedItems, actualItems)
	}
}

// 测试缓存命中率统计
func TestLRU2StoreHitRatio(t *testing.T) {
	opts := Options{
		BucketCount:     4,
		CapPerBucket:    10,
		Level2Cap:       20,
		CleanupInterval: time.Minute,
		OnEvicted:       nil,
	}

	store := newLRU2Cache(opts)
	defer store.Close()

	// 添加50个项
	for i := 0; i < 50; i++ {
		store.Set(fmt.Sprintf("key%d", i), testValue(fmt.Sprintf("value%d", i)))
	}

	// 统计命中次数
	hits := 0
	attempts := 0

	// 尝试获取100个键，一半存在，一半不存在
	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("key%d", i)
		_, found := store.Get(key)
		attempts++
		if found {
			hits++
		}
	}

	// 计算命中率
	hitRatio := float64(hits) / float64(attempts)

	// 验证命中率大致为0.25-0.35（因为我们添加了50个项但有分桶和LRU淘汰）
	if hitRatio < 0.25 || hitRatio > 0.35 {
		t.Errorf("Hit ratio out of expected range: got %.2f", hitRatio)
	}
}

// 测试缓存容量增长和性能
func BenchmarkLRU2StoreOperations(b *testing.B) {
	opts := Options{
		BucketCount:     16,
		CapPerBucket:    1000,
		Level2Cap:       2000,
		CleanupInterval: time.Minute,
		OnEvicted:       nil,
	}

	store := newLRU2Cache(opts)
	defer store.Close()

	// 预填充一些数据
	for i := 0; i < 5000; i++ {
		store.Set(fmt.Sprintf("init-key%d", i), testValue(fmt.Sprintf("value%d", i)))
	}

	b.ResetTimer()

	// 混合操作基准测试
	b.Run("MixedOperations", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			key := fmt.Sprintf("bench-key%d", i%10000)

			// 75%的几率执行Get，25%的几率执行Set
			if i%4 != 0 {
				store.Get(key)
			} else {
				store.Set(key, testValue(fmt.Sprintf("value%d", i)))
			}
		}
	})

	// Get操作基准测试
	b.Run("GetOnly", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			key := fmt.Sprintf("init-key%d", i%5000)
			store.Get(key)
		}
	})

	// Set操作基准测试
	b.Run("SetOnly", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			key := fmt.Sprintf("new-key%d", i)
			store.Set(key, testValue(fmt.Sprintf("value%d", i)))
		}
	})
}

// 辅助函数：检查切片是否包含字符串
func contains(slice []string, str string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}
	return false
}
