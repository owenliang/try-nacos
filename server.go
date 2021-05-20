package main

import (
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"
	"github.com/nacos-group/nacos-sdk-go/clients"
	"github.com/nacos-group/nacos-sdk-go/common/constant"
	"github.com/nacos-group/nacos-sdk-go/vo"
)

func main() {
	// 命令行
	ip := flag.String("ip", "127.0.0.1", "")
	port := flag.Uint64("port", 0, "")
	service := flag.String("service", "", "")
	cluster := flag.String("cluster", "", "")
	flag.Parse()

	// 启动HTTP服务
	r := gin.Default()
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})
	s, err := net.Listen("tcp", fmt.Sprintf(":%v", *port))
	if err != nil {
		panic(err)
	}
	go http.Serve(s, r)
	fmt.Println("启动HTTP服务 finish")

	// 注册服务
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
	if ok, err := client.RegisterInstance(vo.RegisterInstanceParam{
		Ip:          *ip,
		Port:        *port,
		ServiceName: *service,
		Weight:      1,
		Enable:      true,
		Healthy:     true,
		Ephemeral:   true,
		ClusterName: *cluster,
	}); !ok || err != nil {
		panic(err)
	}
	defer client.DeregisterInstance(vo.DeregisterInstanceParam{
		Ip:          *ip,
		Port:        *port,
		ServiceName: *service,
		Ephemeral:   true,
		Cluster:     *cluster,
	})
	fmt.Println("注册服务 finish")

	// 等待退出信号
	sCh := make(chan os.Signal, 1)
	signal.Notify(sCh, syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT)
	<-sCh
	fmt.Println("等待退出信号 finish")
}
