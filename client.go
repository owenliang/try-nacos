package main

import (
	"flag"
	"fmt"
	"time"

	"github.com/nacos-group/nacos-sdk-go/clients"
	"github.com/nacos-group/nacos-sdk-go/common/constant"
	"github.com/nacos-group/nacos-sdk-go/vo"
)

func main() {
	// 命令行
	service := flag.String("service", "", "")
	flag.Parse()

	// 发现服务
	sc := []constant.ServerConfig{
		*constant.NewServerConfig("127.0.0.1", 8848),
	}
	cc := constant.NewClientConfig(
		constant.WithNamespaceId("myns"),
		constant.WithTimeoutMs(5000),
		constant.WithNotLoadCacheAtStart(true),
		constant.WithLogDir("/tmp/nacos/log"),
		constant.WithCacheDir("/tmp/nacos/cache"),
		constant.WithRotateTime("1h"),
		constant.WithMaxAge(3),
		constant.WithLogLevel("debug"),
	)
	client, err := clients.NewNamingClient(vo.NacosClientParam{ClientConfig: cc, ServerConfigs: sc})
	if err != nil {
		panic(err)
	}

	// 获取所有cluster的所有健康节点（根据go nacos sdk实现，大约1秒就能刷新到变化，如果拉回的hosts列表为空则不更新内存hosts列表）
	for {
		instances, err := client.SelectInstances(vo.SelectInstancesParam{
			ServiceName: *service,
			HealthyOnly: true,
		})
		if err != nil {
			fmt.Println("服务发现异常", err)
		} else {
			fmt.Println(instances)
		}
		time.Sleep(1 * time.Second)
	}
}
