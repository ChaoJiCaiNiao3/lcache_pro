package main

import (
	"fmt"

	"github.com/ChaoJiCaiNiao3/lcache_pro"
	"github.com/ChaoJiCaiNiao3/lcache_pro/store"
	"github.com/sirupsen/logrus"
)

func main() {
	server := lcache_pro.NewServer("localhost:50051", "lcache_pro")
	group := lcache_pro.NewGroup("lcache_pro", "localhost:50051", store.NewOptions(), store.LRU)
	picker, err := lcache_pro.NewClientPicker("localhost:50051", "lcache_pro", nil)
	if err != nil {
		logrus.Errorf("failed to create client picker: %v", err)
	}
	lcache_pro.RegisterGroupToServer(group, server)
	lcache_pro.RegisterPeersToServer(picker, server)
	server.Start()
	//开始不断询问操作并打印结果
	for {
		fmt.Println("请输入操作:")
		var operation string
		fmt.Scanln(&operation)
		switch operation {
		case "get":
			fmt.Println("请输入key:")
			var key string
			fmt.Scanln(&key)
			value, err := server.GetFromCacheAndRedis(key)
			if err != nil {
				logrus.Errorf("failed to get: %v", err)
			}
			fmt.Println(value)
		case "set":
			fmt.Println("请输入key:")
			var key string
			fmt.Scanln(&key)
			fmt.Println("请输入value:")
			var value string
			fmt.Scanln(&value)
			server.SetToCacheAndRedis(key, lcache_pro.ByteView(value))
			fmt.Println("设置成功")
		case "delete":
			fmt.Println("请输入key:")
			var key string
			fmt.Scanln(&key)
			server.DeleteCacheAndRedis(key)
			fmt.Println("删除成功")
		case "set_hot":
			fmt.Println("请输入key:")
			var key string
			fmt.Scanln(&key)
			fmt.Println("请输入value:")
			var value string
			fmt.Scanln(&value)
			server.SetToCache(key, lcache_pro.ByteView(value))
			fmt.Println("设置成功")
		case "exit":
			server.Close()
			return
		}
	}
}
