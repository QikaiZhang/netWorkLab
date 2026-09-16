# 实验：抓包看 SSE 是不是"普通 HTTP"

对应实验：`sse/server`。验证目标：SSE 连接里除了一个普通 GET 和一个不结束的 200 响应，没有任何新东西。

## 运行与抓包

```bash
# 终端 1：抓包
sudo tcpdump -i lo0 -nn -A port 8083
# 或 Wireshark 选 lo0，display filter: http（SSE 会被识别为普通 HTTP 响应）

# 终端 2：go run ./sse/server
# 终端 3：
curl -sN --max-time 4 -D - http://localhost:8083/events
# -N 关闭缓冲逐块显示，-D - 打印响应头，--max-time 4 秒后主动断开
```

## 真实响应头（实测）

```text
HTTP/1.1 200 OK
Cache-Control: no-cache
Content-Type: text/event-stream
Date: Wed, 16 Sep 2026 16:47:28 GMT
Transfer-Encoding: chunked
```

三个关键观察：

- `200 OK`：一个再普通不过的成功响应，**没有 101 Switching Protocols**（对比 WebSocket）。
- `Transfer-Encoding: chunked`：响应头里**没有 Content-Length**——因为没人知道流什么时候结束。每个事件（`data: message N\n\n`）就是 HTTP/1.1 的一个 chunk，chunk 头是它的十六进制长度。
- `Content-Type: text/event-stream`：客户端靠它决定"按事件流格式解析 Body"。

## 响应体（实测）

```text
retry: 3000

id: 1
data: message 1

id: 2
data: message 2

id: 3
data: message 3

id: 4
data: message 4
```

每一块都由服务器每秒 flush 一次。Wireshark 的 Follow TCP Stream 里能把这些字节连起来看，时间列则显示它们分布在 4 秒内的多个 chunk 中——**一条连接，多次数据，方向永远是 server → client**。

## 在抓包里确认三件事

1. **握手只有普通 TCP + HTTP**：开头的报文序列是 `SYN / SYN-ACK / ACK / GET /events... / 200 OK...`，与访问普通网页完全一致。
2. **推送数据没有新协议**：后续每秒的 `PSH, ACK` 段，payload 是 `b\r\nid: N\r\ndata: message N\r\n\r\n0\r\n\r\n` 形态的 HTTP chunked 数据（开头的 `b` 是 chunk 长度 0x0b=11）。
3. **断开就是断开 HTTP**：curl 主动断开时出现 `FIN, ACK` 挥手（或 RST），浏览器端则触发 EventSource 自动重连——抓包里会看到**新的** `GET /events` 请求和新的 TCP 连接（这就是重连的真相，没有任何特殊协议动作）。

## 与 WebSocket 抓包对照

| | SSE 抓包 | WebSocket 抓包 |
|---|---|---|
| 建连 | GET + 200 OK | GET（带 Upgrade 头）+ **101 Switching Protocols** |
| 后续报文 | Wireshark 一直解析为 HTTP chunked | Wireshark 解析为 **WebSocket 帧**（opcode/mask） |
| 关闭 | FIN/RST | WebSocket Close 帧 + FIN |

这一对比是回答"SSE 和 WebSocket 有什么区别"最直接的证据：SSE 自始至终是 HTTP；WebSocket 从 101 之后就是另一种协议了。
