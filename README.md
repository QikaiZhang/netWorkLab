# network-lab

> Network protocol learning lab implemented with small experiments and packet captures.

使用 Go 编写最小网络实验，通过 socket 编程、协议实验和 Wireshark 抓包理解 TCP、UDP、HTTP、SSE、TLS、WebSocket、DNS、ICMP、Protobuf 和 gRPC 的工作机制。

## 这个项目是什么 / 不是什么

**是**：一组每个只有 20~100 行的实验程序。代码的作用是"让协议真实发生"。

**不是**：生产级网络框架。这里不会实现完整的 TCP 协议栈、完整 TLS、完整 WebSocket、完整 DNS，也不会做拥塞控制、重传算法或密码学算法。这些内容只在 `docs/` 中解释。

核心方法论只有一条：

```text
写最小代码 → 运行 → 让真实网络通信发生
    → Wireshark / tcpdump 抓包 → 观察真实 Packet → 解释字段
    → 回到 Go 代码 → 解释代码与协议的关系
```

一句话：**代码只是让协议发生，抓包让协议可见，文档负责解释协议。**

## 学习路线

按阶段顺序，每个阶段对应独立的 git commit（见 `git log`）：

| Phase | 主题 | 代码目录 | 概念文档 | 抓包记录 |
|---|---|---|---|---|
| 1 | TCP：echo / 三次握手 / 四次挥手 / SEQ-ACK / 字节流无边界 | `tcp/` | [docs/tcp.md](docs/tcp.md) | [packet/](packet/) |
| 2 | UDP：echo / 超时重传 / 丢包模拟 | `udp/` | [docs/udp.md](docs/udp.md) | [packet/udp.md](packet/udp.md) |
| 3 | HTTP：net/http / 基于 TCP 手写最小 parser | `http/` | [docs/http.md](docs/http.md) | [packet/http.md](packet/http.md) |
| 4 | SSE：text/event-stream 长连接推送 | `sse/` | [docs/sse.md](docs/sse.md) | [packet/sse.md](packet/sse.md) |
| 5 | TLS：握手观察 / TLS 1.2 vs 1.3 | `tls/` | [docs/tls.md](docs/tls.md) | [packet/tls-handshake.md](packet/tls-handshake.md) |
| 6 | WebSocket：Upgrade / 101 / Frame | `websocket/` | [docs/websocket.md](docs/websocket.md) | [packet/websocket.md](packet/websocket.md) |
| 7 | DNS：LookupHost / 手工构造 UDP DNS 查询 | `dns/` | [docs/dns.md](docs/dns.md) | [packet/dns.md](packet/dns.md) |
| 8 | ICMP：Echo Request / Reply | `icmp/` | [docs/icmp.md](docs/icmp.md) | [packet/icmp.md](packet/icmp.md) |
| 9 | Protobuf：schema / 序列化 / 与 JSON 对比 | `proto/` | [docs/proto.md](docs/proto.md) | - |
| 10 | gRPC：最小服务 / HTTP/2 | `grpc/` | [docs/grpc.md](docs/grpc.md) | - |
| - | 协议总图 + 请求全过程 | - | [docs/architecture.md](docs/architecture.md) | - |
| - | 校招面试题整理 | - | [docs/interview.md](docs/interview.md) | - |

## 目录结构

```text
network-lab/
├── tcp/        echo 与字节流（粘包/半包）实验
├── udp/        echo 与超时重传实验
├── http/       net/http 服务 + 基于 TCP 的最小 HTTP parser
├── sse/        text/event-stream 推送 + EventSource 演示页
├── tls/        自签证书生成 + HTTPS server/client
├── websocket/  gorilla/websocket echo + Ping/Pong/Close
├── dns/        LookupHost + 手工构造/解析 UDP DNS 报文
├── icmp/       非特权 ICMP datagram socket ping
├── proto/      .proto + 生成代码 + 序列化对比 JSON
├── grpc/       .proto + 生成代码 + 最小 Unary 服务
├── packet/     每个实验的抓包现场记录
└── docs/       每个协议的概念文档 + 总图 + 面试题
```

git 历史就是学习路线：每个阶段在独立分支（`phase-N-*`）上开发，完成后合入 main；`git log` 从上到下即是推荐学习顺序。Phase 9/10 的 `*.pb.go` 为 protoc 生成代码，已提交以保证开箱可跑；修改 `.proto` 后重新生成：

```bash
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
protoc --go_out=. --go_opt=module=network-lab proto/user.proto
protoc --go_out=. --go_opt=module=network-lab \
       --go-grpc_out=. --go-grpc_opt=module=network-lab grpc/user.proto
```

## 如何运行

所有实验都在本仓库根目录下以 `go run` 启动，先起 server（或只起 client），另开一个终端跑 client：

```bash
# Phase 1: TCP echo
go run ./tcp/echo/server
go run ./tcp/echo/client

# Phase 1: TCP 字节流（粘包/半包）演示：先起 server，client 会自动跑两种写入模式
go run ./tcp/stream/server
go run ./tcp/stream/client

# Phase 2: UDP echo / 超时重传
go run ./udp/echo/server
go run ./udp/echo/client

go run ./udp/retry/server
go run ./udp/retry/client

# Phase 3: HTTP
go run ./http/server          # curl http://localhost:8081/hello
go run ./http/minparser/server
go run ./http/minparser/client

# Phase 4: SSE
go run ./sse/server           # curl -N http://localhost:8083/events 或浏览器打开 http://localhost:8083/

# Phase 5: TLS（先生成自签证书，gitignore 不入库）
go run ./tls/gencert
go run ./tls/server
go run ./tls/client

# Phase 6: WebSocket
go run ./websocket/server
go run ./websocket/client

# Phase 7: DNS
go run ./dns/lookup example.com
go run ./dns/rawquery example.com

# Phase 8: ICMP（macOS 用非特权 ICMP datagram socket，无需 root；详见 docs/icmp.md）
go run ./icmp/ping 127.0.0.1

# Phase 9: Protobuf
go run ./proto

# Phase 10: gRPC（两个终端）
go run ./grpc/server
go run ./grpc/client
```

## 如何使用 Wireshark / tcpdump

### 安装

```bash
# macOS
brew install --cask wireshark
# 安装包内同时带有命令行工具 tshark；也可以只用系统自带的 tcpdump
```

### 选择网卡

本机实验全部监听 `localhost`，所以抓包必须选**回环接口**：

- macOS：`Loopback: lo0`
- Linux：`lo`
- Windows：`Adapter for loopback traffic capture`（依赖 Npcap）

### Capture Filter 与 Display Filter

Wireshark 有两套过滤器，语法不同，别混用：

- **Capture Filter**（抓包前设置，BPF 语法，决定抓什么）：如 `tcp port 8080`
- **Display Filter**（抓完后过滤显示，本仓库文档中提到的过滤器都指这种）

### 常用 Display Filter

| 目的 | Display Filter |
|---|---|
| 指定端口的 TCP 流量 | `tcp.port == 8081` |
| 指定端口的 UDP 流量 | `udp.port == 9002` |
| 纯 SYN（第一次握手） | `tcp.flags.syn == 1 && tcp.flags.ack == 0` |
| 所有带 SYN 的包（含 SYN+ACK） | `tcp.flags.syn == 1` |
| 带 FIN 的包 | `tcp.flags.fin == 1` |
| HTTP | `http` |
| TLS | `tls`（Wireshark 3.0 之前版本叫 `ssl`，旧过滤器写法是 `ssl`） |
| WebSocket | `websocket` |
| DNS | `dns` |
| ICMP | `icmp` |

版本差异说明：`tls` 过滤器在 Wireshark 3.0+ 才有效，老版本请用 `ssl`；`websocket` 过滤器随版本对分片帧的展示粒度略有不同，若某条过滤器报错（显示红色/粉色底），先检查 Wireshark 版本。

### tcpdump 等价命令

```bash
# 抓回环上 8080 端口，不解析主机名/端口，同时打印 ASCII
sudo tcpdump -i lo0 -nn -A port 8081

# 抓 SYN（第一次握手）与 FIN
sudo tcpdump -i lo0 -nn 'tcp[tcpflags] & (tcp-syn|tcp-fin) != 0'

# 保存成 pcap 文件后用 Wireshark 打开
sudo tcpdump -i lo0 -nn port 8443 -w out.pcap
```

### 权限说明

- `tcpdump` / Wireshark 抓包需要 root（`sudo tcpdump ...`），或安装 Wireshark 时勾选 ChmodBPF 组件。
- 本仓库的 `icmp` 实验默认使用 macOS 支持的**非特权 ICMP datagram socket**（`x/net/icmp`），无需 root；若失败可参考 `docs/icmp.md` 用 `sudo` + raw socket 方式运行。

### 实用技巧

- 选中一条 TCP 报文后右键 **Follow → TCP Stream**，可以看到整条连接的应用层字节内容（HTTP、SSE 都靠它看）。
- Wireshark 默认显示相对 Sequence Number（从 0 开始），观察 SEQ/ACK 实验时更直观。

## 文档地图

- `docs/*.md`：每个协议"是什么、解决什么问题、在哪一层、谁替你做了"，以及协议之间的关系。
- `packet/*.md`：每个实验的抓包现场——怎么抓、应该看到什么、每个字段怎么读。
- `docs/architecture.md`：把所有协议叠成一张分层总图 + 浏览器访问 `https://example.com` 等请求全过程。
- `docs/interview.md`：按"一句话回答 → 原理 → 实验 → 抓包 → 面试展开"格式整理的校招复习题。
