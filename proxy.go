package main

import (
	"flag"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/nacos-group/nacos-sdk-go/clients"
	"github.com/nacos-group/nacos-sdk-go/clients/naming_client"
	"github.com/nacos-group/nacos-sdk-go/common/constant"
	"github.com/nacos-group/nacos-sdk-go/model"
	"github.com/nacos-group/nacos-sdk-go/vo"
)

// HTTP反向动态代理
type ProxyHandler struct {
	client    naming_client.INamingClient // nacos客户端
	transport *http.Transport             // http客户端
}

func newProxyHandler(client naming_client.INamingClient) (proxyHandler *ProxyHandler) {
	proxyHandler = &ProxyHandler{client: client}
	proxyHandler.transport = &http.Transport{
		DisableKeepAlives: true, // 禁止连接池
	}
	return
}

func (proxyHandler *ProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var err error

	// 从header中提取出domain
	host := r.Host
	domain := strings.Split(host, ":")[0]

	// TODO：当调用某个节点失败，需要有重试机制

	// 查找Nacos可用地址
	var instance *model.Instance
	instance, err = proxyHandler.client.SelectOneHealthyInstance(vo.SelectOneHealthInstanceParam{ServiceName: domain})

	// 目标地址
	var remoteUrl *url.URL
	if err != nil { // 服务发现失败，走正常域名解析
		remoteUrl, err = url.Parse(fmt.Sprintf("http://%v", host))
	} else { // 否则填为服务发现IP+PORT
		remoteUrl, err = url.Parse(fmt.Sprintf("http://%v:%v", instance.Ip, instance.Port))
	}

	// 有错误
	if err != nil {
		w.WriteHeader(500)
		return
	}

	// 进行转发
	proxy := httputil.NewSingleHostReverseProxy(remoteUrl)
	rawDirector := proxy.Director
	proxy.Director = func(r *http.Request) { // 为转发请求附加header
		rawDirector(r)
		r.Header.Set("Host", host) // 保持原始host
	}
	proxy.ServeHTTP(w, r)
}

// 检查应用端口存活
func isApplicationAlive(port uint64) (alive bool) {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%v", port), 1*time.Second)
	if err == nil {
		alive = true
		conn.Close()
	}
	return
}

var (
	ip      = flag.String("ip", "127.0.0.1", "应用的IP")
	port    = flag.Uint64("port", 0, "应用的PORT")
	service = flag.String("service", "", "应用的Service")
	cluster = flag.String("cluster", "", "应用的Cluter")
)

func main() {
	// 命令行
	flag.Parse()

	// 连接nacos server
	sc := []constant.ServerConfig{
		*constant.NewServerConfig("127.0.0.1", 8848), // 测试用的nacos server跑在本地
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
	fmt.Println("连接nacos server finish")

	// 启动HTTP反向代理服务
	s, err := net.Listen("tcp", ":1500") // proxy固定监听在1500端口
	if err != nil {
		panic(err)
	}
	go http.Serve(s, &ProxyHandler{client: client})
	fmt.Println("启动HTTP反向代理服务 finish")

	// 等待应用ready
	for {
		if isApplicationAlive(*port) {
			break
		}
		time.Sleep(1 * time.Second)
		fmt.Println("等待应用ready trying...")
	}
	fmt.Println("等待应用ready finish")

	// 注册服务
	if ok, err := client.RegisterInstance(vo.RegisterInstanceParam{
		Ip:          *ip,
		Port:        *port,
		ServiceName: *service,
		Weight:      1,
		Healthy:     true,
		Enable:      true,
		Ephemeral:   true,
		ClusterName: *cluster,
	}); !ok || err != nil {
		panic(err)
	}
	// 退出前主动解绑注册
	defer func() {
		client.DeregisterInstance(vo.DeregisterInstanceParam{
			Ip:          *ip,
			Port:        *port,
			ServiceName: *service,
			Ephemeral:   true,
			Cluster:     *cluster,
		})
		fmt.Println("解除注册 finish")
	}()
	fmt.Println("注册服务 finish")

	// 监听退出信号
	sCh := make(chan os.Signal, 1)
	signal.Notify(sCh, syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT)

	// 反复检查应用存活
	alive := true
	for {
		nowAlive := isApplicationAlive(*port)
		// 健康 -> 不健康
		if alive && !nowAlive {
			alive = false
			for {
				if ok, err := client.RegisterInstance(vo.RegisterInstanceParam{
					Ip:          *ip,
					Port:        *port,
					ServiceName: *service,
					Weight:      1,
					Healthy:     true,
					Enable:      false, // 这个字段能控制节点上下线
					Ephemeral:   true,
					ClusterName: *cluster,
				}); !ok || err != nil {
					time.Sleep(1 * time.Second)
					continue
				}
				break
			}
			fmt.Println("健康->不健康 finish")
		} else if !alive && nowAlive { // 不健康 -> 健康
			alive = true
			for {
				if ok, err := client.RegisterInstance(vo.RegisterInstanceParam{
					Ip:          *ip,
					Port:        *port,
					ServiceName: *service,
					Weight:      1,
					Healthy:     true,
					Enable:      true, // 这个字段能控制节点上下线
					Ephemeral:   true,
					ClusterName: *cluster,
				}); !ok || err != nil {
					time.Sleep(1 * time.Second)
					continue
				}
				break
			}
			fmt.Println("不健康->健康 finish")
		}
		time.Sleep(1 * time.Second)

		// 判断退出信号
		select {
		case <-sCh:
			fmt.Println("等待退出信号 finish")
			goto END
		default:
		}
	}
END:
}
