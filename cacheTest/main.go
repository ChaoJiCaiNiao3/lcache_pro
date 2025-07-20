package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/ChaoJiCaiNiao3/lcache_pro"
	"github.com/ChaoJiCaiNiao3/lcache_pro/store"
	"github.com/sirupsen/logrus"
)

func main() {
	//读取端口
	fmt.Println("请输入端口:")
	var port string
	fmt.Scanln(&port)
	//启动服务
	server := lcache_pro.NewServer("localhost:"+port, "lcache_pro")
	group := lcache_pro.NewGroup("lcache_pro", "localhost:"+port, store.NewOptions(), store.LRU)
	picker, err := lcache_pro.NewClientPicker("localhost:"+port, "lcache_pro", nil)
	if err != nil {
		logrus.Errorf("failed to create client picker: %v", err)
	}
	lcache_pro.RegisterGroupToServer(group, server)
	lcache_pro.RegisterPeersToServer(picker, server)
	server.Start()
	reader := bufio.NewReader(os.Stdin)
	//开始不断询问操作并打印结果
	for {
		fmt.Println("请输入操作:")
		operation, _ := reader.ReadString('\n')
		operation = strings.TrimRight(operation, "\r\n")
		switch operation {
		case "get":
			fmt.Println("请输入key:")
			key, _ := reader.ReadString('\n')
			key = strings.TrimRight(key, "\r\n")
			value, err := server.GetFromCacheAndRedis(key)
			if err != nil {
				logrus.Errorf("failed to get: %v", err)
			}
			fmt.Println(value)
		case "set":
			fmt.Println("请输入key:")
			key, _ := reader.ReadString('\n')
			key = strings.TrimRight(key, "\r\n")
			fmt.Println("请输入value:")
			value, _ := reader.ReadString('\n')
			value = strings.TrimRight(value, "\r\n")
			server.SetToCacheAndRedis(key, lcache_pro.ByteView(value))
			fmt.Println("设置成功")
		case "delete":
			fmt.Println("请输入key:")
			key, _ := reader.ReadString('\n')
			key = strings.TrimRight(key, "\r\n")
			server.DeleteCacheAndRedis(key)
			fmt.Println("删除成功")
		case "set_hot":
			fmt.Println("请输入key:")
			key, _ := reader.ReadString('\n')
			key = strings.TrimRight(key, "\r\n")
			fmt.Println("请输入value:")
			value, _ := reader.ReadString('\n')
			value = strings.TrimRight(value, "\r\n")
			server.SetToCache(key, lcache_pro.ByteView(value))
			fmt.Println("设置成功")
		case "exit":
			server.Close()
			return
		}
	}
}
