# 网络协议总图与请求全过程

> 本文档把所有实验连接成一张图。每个协议的细节见对应 `docs/*.md`，每个实验的抓包现场见 `packet/*.md`。

## 一、总体分层图

```text
┌─────────────────────────────── 应用层 ────────────────────────────────┐
│                                                                       │
│  HTTP/1.1  HTTP/2   ← 传输对象是"请求/响应消息"                        │
│    │            │                                                     │
│    │            └────── gRPC（RPC 调用模型 + Protobuf 编码）            │
│    │                                                                  │
│    ├── SSE（借 HTTP 长响应体做 Server→Client 推送）                     │
│    ├── WebSocket（先 HTTP Upgrade，再切换成独立帧协议）                 │
│    └── DNS（域名→IP，通常跑在 UDP 53 上）                              │
│                                                                       │
│  Protobuf：不是协议，是序列化格式（可跑在 HTTP body / gRPC / 裸 TCP 上） │
│  TLS：    不是应用协议，是给 TCP 流加"机密性/完整性/身份认证"的保护层    │
└───────────────────────────────────────────────────────────────────────┘
                        │
┌─────────────────── 传输层 ──────────────────┐
│  TCP：面向连接、可靠、有序、字节流（无边界）  │
│  UDP：无连接、不可靠、数据报（有边界）        │
│  （端口号：进程寻址）                        │
└─────────────────────────────────────────────┘
                        │
┌─────────────────── 网络层 ──────────────────┐
│  IP：寻址 + 路由，尽力而为                   │
│  ICMP：IP 的控制面（ping / 差错 / traceroute）│
│  （无端口概念）                              │
└─────────────────────────────────────────────┘
                        │
                    链路层（以太网 / Wi-Fi，抓包最底层可见）
```

记忆锚点：

- **TCP 与 UDP 是所有应用协议的两条腿**；选谁取决于要不要"可靠"。
- **HTTP/SSE/WebSocket/DNS/gRPC 全部是应用层约定**，它们自己不传输任何字节。
- **TLS 不是一层协议栈那么神秘**：它就是"把 TCP 的字节流加密后再交给应用"。

## 二、六个典型协议栈

### HTTPS（浏览器访问网站）

```text
HTTP          ← 请求/响应
 ↓
TLS           ← 加密/证书认证（TLS 1.3 握手 1-RTT）
 ↓
TCP           ← 可靠传输
 ↓
IP
```

### WebSocket（双向实时通道）

```text
WebSocket     ← 帧协议（Text/Binary/Ping/Pong/Close）
 ↓
HTTP Upgrade  ← 只在握手时存在，101 之后退出舞台
 ↓
TCP
 ↓
IP
```

### SSE（服务端单向推送）

```text
SSE           ← 只是事件流的文本格式约定
 ↓
HTTP          ← 一个永不结束的响应体（chunked）
 ↓
TCP
 ↓
IP
```

### gRPC

```text
gRPC          ← RPC 调用模型（方法路由/状态码/超时/流模式）
 ↓
Protobuf      ← 消息编码（+ 5 字节长度前缀）
 ↓
HTTP/2        ← 多路复用 stream / HPACK / trailer
 ↓
TLS（通常）
 ↓
TCP
 ↓
IP
```

### DNS

```text
DNS           ← 查询/响应报文（Header/Question/Answer）
 ↓
UDP 53        ← 小报文一问一答；大响应/DoT/DoH 用 TCP
 ↓
IP
```

### Ping

```text
ICMP Echo     ← Type 8 / Type 0，无端口
 ↓
IP            ← Protocol=1，与 TCP/UDP 平级
```

## 三、请求全过程：浏览器访问 https://example.com

```text
1. DNS 解析
   浏览器缓存 → OS 缓存 → 本地 DNS（递归：根 → .com TLD → example.com 权威）
   → 得到 104.x.x.x（A 记录），按 TTL 缓存
   [实验: dns/lookup, dns/rawquery | 抓包: packet/dns.md]

2. TCP 三次握手（到 443 端口）
   SYN → SYN+ACK → ACK；双方进入 ESTABLISHED
   此刻还没有任何 TLS/HTTP 字节
   [实验: tcp/echo | 抓包: packet/tcp-handshake.md]

3. TLS 1.3 握手（1-RTT）
   ClientHello（套件列表 + key_share + SNI=example.com）
   ← ServerHello（选定套件 + key_share）
   ← {EncryptedExtensions, Certificate, CertificateVerify, Finished}（已加密）
   → Finished
   → 双方得到会话密钥，之后的字节全部加密
   [实验: tls/server+client | 抓包: packet/tls-handshake.md]

4. HTTP 请求（加密的 Application Data）
   GET / HTTP/1.1（或 ALPN 协商出的 HTTP/2）
   Host: example.com

5. HTTP 响应
   200 OK + Content-Length/chunked + HTML 字节
   （keep-alive：连接不关，下一个请求继续用）
   [实验: http/server | 抓包: packet/http.md]

6. TCP 关闭（连接空闲超时或浏览器关闭时）
   FIN → ACK → FIN → ACK；主动关闭方进入 TIME_WAIT
   [抓包: packet/tcp-close.md]
```

内核与标准库的分界：以上 2/3/6 全部由内核协议栈 + crypto/tls 完成，Go/浏览器代码只发起 DNS 查询和 HTTP 消息。

## 四、请求全过程：浏览器建立 WebSocket

```text
1. DNS → IP（与 HTTPS 完全相同：先解析出 ws/wss 主机的地址）
2. TCP 握手（80 或 443）
3. （wss:// 时先 TLS 握手）
4. HTTP Upgrade 请求
   GET /ws HTTP/1.1
   Upgrade: websocket
   Sec-WebSocket-Key: ...
5. 101 Switching Protocols（Sec-WebSocket-Accept 校验）
6. 之后同一条 TCP 连接上跑 WebSocket 帧：
   Text/Binary 帧（带掩码的客户端帧）、Ping/Pong 心跳、Close 帧告别
7. Close 帧之后 → TCP 四次挥手
   [实验: websocket/server+client | 抓包: packet/websocket.md]
```

## 五、请求全过程：SSE

```text
1. DNS / TCP / TLS（https 时）同上
2. 普通 HTTP 请求：GET /events
3. 普通 HTTP 响应：200 OK
   Content-Type: text/event-stream
   Transfer-Encoding: chunked（没有 Content-Length——流没有尽头）
4. 服务器持续写事件并 flush：
   id: 1\ndata: message 1\n\n   ← 一个 HTTP chunk
   id: 2\ndata: message 2\n\n
   ...（连接永不完成）
5. 断线时浏览器 EventSource 自动重连（retry: 毫秒数 + Last-Event-ID 请求头）
   重连 = 全新的一轮 TCP(→TLS)→HTTP
   [实验: sse/server | 抓包: packet/sse.md]
```

## 六、"OS 与标准库替你做了什么"总表

| 你写的 | 内核/标准库做的 |
|---|---|
| `net.Dial("tcp", addr)` | connect(2)：SYN/SYN+ACK/ACK、重传、路由 |
| `conn.Write` | 拷贝进发送缓冲、分段(≤MSS)、SEQ/ACK、重传、拥塞控制 |
| `conn.Read` | 从接收缓冲取字节、乱序重排、去重 |
| `conn.Close` | 发完缓冲区数据、FIN、四次挥手、TIME_WAIT |
| `net.ListenUDP/WriteToUDP` | UDP 头、校验和，别无所有 |
| `http.ListenAndServe` | TCP accept 循环、HTTP 解析、chunked 编码、keep-alive、并发 |
| `http.ListenAndServeTLS` | 上述全部 + TLS 握手/加解密 |
| `websocket.Upgrader.Upgrade` | 校验握手头、101、帧编解码、掩码 |
| `proto.Marshal` | 按 field number + wire type 编码 |
| `client.GetUser(ctx, req)` | HTTP/2 stream、HPACK、gRPC 前缀、超时传播 |

每个实验都在验证这张表的某一行——**理解了"谁做了什么"，就理解了网络分层**。
