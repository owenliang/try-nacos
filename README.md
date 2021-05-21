# try-nacos

## 启动nacos server（单机版）

```
bin/startup.sh -m standalone
```

## 服务注册（侵入式）

server.go演示了服务端程序如何注册到nacos server。

下面启动2个server.go进程，模拟分别注册到main和backup机房，同时对外提供a.yuerblog.cc的服务：

```
启动main机房的进程：
go run server.go -ip 127.0.0.1 -port 8765 -service a.yuerblog.cc -cluster main

启动backup机房的进程：
go run server.go -ip 127.0.0.1 -port 8764 -service a.yuerblog.cc -cluster backup

client.go演示了如何通过服务发现，得到a.yuerblog.cc的上述2个进程地址：
go run client.go  -service a.yuerblog.cc
```

## 服务注册（非侵入式）

PHP/PYTHON等应用不适合侵入式服务注册&发现，因此我们利用proxy.go为应用提供HTTP反向代理。

应用框架需要通过hook的思路将HTTP RPC改写给proxy.go服务，由proxy.go访问nacos server完成服务发现与HTTP转发。

另外，proxy.go会帮助应用完成服务注册，它通过探测应用端口来实时更新Nacos server中该进程的状态。

```
启动proxy.go，代替应用注册地址为(127.0.01,8765)的服务进程，其属于b.yuerblog.cc服务，集群属于main：
go run proxy.go -ip 127.0.0.1 -port 8765 -service b.yuerblog.cc -cluster main

proxy.go监听在1500端口，我们可以通过它反向代理访问其他服务（例如a.yuerblog.cc)：
curl localhost:1500/ping -H 'Host:a.yuerblog.cc' 

也可以访问没有注册的域名，此时Proxy.go会走普通域名解析转发：
curl localhost:1500/ping -H 'Host:baidu.com'
```

## 容灾测试

1，nacos server故障期间，客户端仍旧本地缓存的hosts列表。
2，客户端主动取消注册，则其他客户端1秒后会刷新本地hosts缓存，这是客户端主动pull的（感觉集群规模大了性能要坑）。
3，客户端意外退出，则客户端持续30秒没有向nacos server心跳，最终nacos server会剔除该host。(GO客户端没法设置这个剔除时间，需要关注一下)