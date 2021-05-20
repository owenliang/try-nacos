# try-nacos

## 启动nacos

bin/startup.sh -m standalone

## 服务注册

模拟a.yuerblog.cc服务，在2个机房同时提供服务。

启动main机房的进程：
go run server.go -ip 127.0.0.1 -port 8765 -service a.yuerblog.cc -cluster main

启动backup机房的进程：
go run server.go -ip 127.0.0.1 -port 8764 -service a.yuerblog.cc -cluster backup

## 服务发现

会发现上述2个地址，可以负载均衡调用，也可以做自己的本机房优先策略。

go run client.go  -service a.yuerblog.cc

## 容灾测试

1，杀死nacos，不影响服务发现的hosts列表，直到nacos重新上线，一切正常。
2，杀死某个服务进程，1秒后客户端能更新到最新的hosts列表。
3，服务进程意外退出（即不主动取消注册），默认30秒后nacos才会剔除它，这些配置发现客户端侧不能改，可能在UI里吧。