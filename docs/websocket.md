# WebSocket 概念文档

> 配套实验：`websocket/server`、`websocket/client`，抓包现场见 [packet/websocket.md](../packet/websocket.md)。

## WebSocket 在哪一层

WebSocket 是**应用层**协议，但它比较特殊：出生时是 HTTP（握手阶段），`101 Switching Protocols` 之后**蜕变为一个独立的双向帧协议**，继续跑在同一条 TCP 连接上。

```text
WebSocket（帧协议，全双工）
 ↓
HTTP Upgrade（只负责"借道"完成握手）
 ↓
TCP
 ↓
IP
```

## 为什么需要 Upgrade

WebSocket 出现（2011 年 RFC 6455）时，Web 的世界已经全是 HTTP 基础设施：防火墙、代理、负载均衡都认识 80/443 端口和 HTTP 语义。让浏览器直接开一个非 HTTP 协议的 TCP 连接，会被大量中间设备拦掉。

于是设计成：

1. 先发一个**长得完全像 HTTP 的请求**，带上 `Upgrade: websocket`。
2. 服务器如果支持，回 `101 Switching Protocols`。
3. 从此刻起，这条 TCP 连接上**不再有 HTTP**——两边开始说 WebSocket 帧协议。

"借道 HTTP 穿过基础设施，然后切换协议"——这就是 Upgrade 的全部意义。

## 握手长什么样（实测）

请求（必须的头部）：

```text
GET /ws HTTP/1.1
Host: localhost:8084
Upgrade: websocket
Connection: Upgrade
Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==
Sec-WebSocket-Version: 13
```

响应：

```text
HTTP/1.1 101 Switching Protocols
Upgrade: websocket
Connection: Upgrade
Sec-WebSocket-Accept: s3pPLMBiTxaQ9kYGzzhZRbK+xOo=
```

- `Sec-WebSocket-Key` 是客户端随机 base64；`Sec-WebSocket-Accept = base64(SHA1(key + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))`。它不是加密，只是证明"对方真的懂 WebSocket，而不是恰好回了个 101 的代理"。
- `wss://` 版本 = 同样的握手跑在 TLS 之上：`wss = WebSocket over TLS over TCP`。

## Frame：切换后的语言

从 101 之后，线路上全是 WebSocket 帧（RFC 6455）：

```text
 0                   1                   2                   3
 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
+-+-+-+-+-------+-+-------------+-------------------------------+
|F|R|R|R| opcode|M| Payload len |    Extended payload length    |
|I|S|S|S|  (4)  |A|     (7)     |          (16/64 bits)         |
|N|V|V|V|       |S|             |                               |
+-+-+-+-+-------+-+-------------+ - - - - - - - - - - - - - - - +
|     Masking-key（客户端→服务端的帧必须带，4 字节）             |
+-------------------------------+-------------------------------+
|                     Payload Data（应用数据）                   |
+---------------------------------------------------------------+
```

- **opcode**：`0x1` Text（UTF-8）、`0x2` Binary、`0x9` Ping、`0xA` Pong、`0x8` Close、`0x0` Continuation（长消息分片）。
- **Payload len**：7 bit 不够就再用 16 bit，再不够用 64 bit——WebSocket 在帧头层面解决了消息边界问题（对比 TCP 的字节流）。
- **Masking**：客户端发出的帧必须用随机 key 异或掩码，防止中间代理缓存被"投毒"（历史安全设计）；服务端→客户端的帧不掩码。
- **Ping/Pong**：心跳保活 + 探测对端存活。收到 Ping 必须尽快回 Pong。gorilla 里服务端自动回 Pong，客户端需要自己实现 ping 周期与读超时。
- **Close**：关闭也走帧（带状态码，如 `1000 normal closure`），不是直接断 TCP——先协议层告别，再 TCP 挥手。

## 全双工

HTTP 的请求-响应是半双工（一问一答）；WebSocket 建立后，**双方随时可以主动发帧**，就像 TCP 本身一样——但应用层拿到的是"带边界的消息"而不是字节流。这是聊天室、行情推送、协同编辑需要它的根本原因。

## 与 SSE 的选择

| 需求 | 选择 |
|---|---|
| 只是服务端单向推（通知/进度/行情） | SSE（纯 HTTP，浏览器自动重连，简单） |
| 双向实时（聊天/游戏/协同） | WebSocket |
| 客户端偶尔上报、主要是收 | SSE + 普通 POST 也够 |

## Go 代码与协议的关系

- `upgrader.Upgrade(w, r, nil)`：校验 Upgrade 头、计算 Sec-WebSocket-Accept、回 101、接管 `http.Hijacker` 拿到裸 TCP 连接——从此这条连接绕开 http.Server 的请求循环。
- `conn.ReadMessage/WriteMessage`：自动处理帧的编解码、掩码、分片；你面对的是完整消息（和 TCP 的 Read 形成对照）。
- gorilla 自动响应 Ping（Pong）、自动校验 Close 握手；应用只需要关心 Text/Binary 帧。
- 浏览器端 `new WebSocket('ws://host/ws')` 做的事与此相同：握手、事件回调（onopen/onmessage/onclose）。
