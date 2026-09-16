# gRPC 概念文档

> 配套实验：`grpc/server`、`grpc/client`。

## Proto 和 gRPC 不是同一个东西

```text
.proto 文件
    │
    ├── message 部分 → 只描述数据结构 + 序列化（Protobuf 的领域）
    │
    └── service 部分 → 描述"远程方法"，由 gRPC 框架实现调用语义
```

- **Protobuf** 负责：数据长什么样、怎么变成字节。
- **gRPC** 负责：怎么把一次"方法调用"变成一次网络通信——方法路由、请求/响应配对、超时、状态码、流式传输，全部构建在 HTTP/2 之上。

```text
gRPC
 ↓
HTTP/2（多路复用的帧 + HPACK 头部）
 ↓
TCP
 ↓
IP
```

（通常还有一层 TLS：`gRPC over HTTP/2 over TLS`；本实验用明文 h2c 以便抓包。）

## 生成流水线

```text
user.proto
   ↓  protoc --go_out  --go-grpc_out
user.pb.go        数据结构 + 序列化（来自 message 定义）
user_grpc.pb.go   Client 接口 + Server 接口 + 方法路由（来自 service 定义）
   ↓
server: 注册实现    pb.RegisterUserServiceServer(s, &server{})
client: 像本地方法一样调用   client.GetUser(ctx, &pb.GetUserRequest{Id: 42})
```

生成的代码做的事就是把"调用 GetUser"翻译成：往 `:50051` 发一个 HTTP/2 POST。生成命令：

```bash
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
protoc --go_out=. --go_opt=module=network-lab \
       --go-grpc_out=. --go-grpc_opt=module=network-lab \
       grpc/user.proto
```

## gRPC 在 HTTP/2 上到底发了什么

Go 的 `GODEBUG=http2debug=2` 能打印 HTTP/2 帧。本仓库 `GODEBUG=http2debug=2 go run ./grpc/client` 的实测输出（节选）：

```text
http2: Framer: wrote SETTINGS len=0                       ← 连接预检，双方交换参数
http2: Framer: wrote HEADERS flags=END_HEADERS stream=1   ← "HTTP 请求"其实是 RPC
http2: decoded hpack field ":status" = "200"
http2: decoded hpack field "content-type" = "application/grpc"
http2: Framer: read HEADERS flags=END_STREAM stream=1 len=24
http2: decoded hpack field "grpc-status" = "0"            ← RPC 结果放在 trailer 里
http2: decoded hpack field "grpc-message" = ""
rpc returned: id=42 name=user-42
http2: Framer: wrote GOAWAY ... "client transport shutdown"
```

对应到 HTTP 语义，一次 `GetUser(id=42)` 就是：

```text
POST /user.UserService/GetUser HTTP/2
content-type: application/grpc
te: trailers

<5 字节 gRPC 前缀><GetUserRequest 的 protobuf 字节>    ← DATA 帧

HTTP/2 200
content-type: application/grpc

<5 字节 gRPC 前缀><GetUserResponse 的 protobuf 字节>   ← DATA 帧
grpc-status: 0                                         ← HEADERS 尾帧（trailer）
grpc-message: ""
```

四个关键观察：

1. **`:path` 就是方法全名** `包名.服务名/方法名`——gRPC 的"方法路由"只是 HTTP/2 路径匹配。
2. **请求体 = Protobuf 字节 + 5 字节前缀**（1 字节压缩标志 + 4 字节大端长度）。这是 length-prefix 消息边界方案的又一次出现（见 [packet/tcp-byte-stream.md](../packet/tcp-byte-stream.md)）——gRPC 在 HTTP/2 帧之上还需要它，因为一条 stream 里可能有多条消息（流式 RPC）。
3. **响应状态在 trailer 里**：HTTP 头先给 `200`（"传输成功"），真正的 RPC 结果 `grpc-status: 0`（"业务成功"）在响应**末尾**的 trailer 帧中——因为服务端处理到一半时还不知道业务成败。HTTP/1.1 没有 trailer 机制，这是 gRPC 必须 HTTP/2 的原因之一。
4. **HTTP/2 stream=1**：多条并发 RPC 各占一条 stream，互不阻塞地跑在同一条 TCP 连接上——这就是多路复用，解决了 HTTP/1.1 的连接数问题与队头排队。

## 为什么是 HTTP/2

- **多路复用**：一条连接并发跑成百上千个 RPC（HTTP/1.1 每请求一条连接）。
- **双向流**：HTTP/2 的 stream 本身支持双向数据，四种 RPC 模式里 streaming 三种都靠它。
- **HPACK 头部压缩**：重复的 metadata 不再每次全量发送。
- **强制 trailer**：承载 grpc-status。
- 注意：HTTP/2 只解决了 HTTP 层队头阻塞；TCP 层的队头阻塞（一个段丢，所有 stream 等）仍在，这正是 HTTP/3/QUIC 想解决的（多路复用挪到 UDP 上自己实现）。

## 四种 RPC 模式

| 模式 | 请求/响应 | 场景 |
|---|---|---|
| Unary | 1 → 1 | 本实验、普通 CRUD |
| Server streaming | 1 → N | 推送行情/日志（对比 SSE） |
| Client streaming | N → 1 | 上传分块、批量上报 |
| Bidi streaming | N ↔ N | 聊天、实时协同（对比 WebSocket） |

## Go 代码与协议的关系

- `grpc.NewServer()` 内含一个 HTTP/2 server；`grpc.NewClient` 内含 HTTP/2 client 与连接池。**你写的 handler 只处理 protobuf 消息**，HTTP/2 帧、HPACK、stream 管理、5 字节前缀全部由框架完成。
- `ctx` 贯穿超时/取消：client 的 `context.WithTimeout` 会转成 HTTP/2 的 `grpc-timeout` 头传给服务端——分布式调用的 deadline 传播。
- 生产环境加 `grpc.WithTransportCredentials(credentials.NewTLS(...))`，即为 gRPC over TLS。
