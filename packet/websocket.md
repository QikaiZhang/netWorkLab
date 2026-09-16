# 实验：抓包看 Upgrade 与 WebSocket 帧

对应实验：`websocket/server`、`websocket/client`。观察目标：101 前是 HTTP，101 后是 WebSocket 帧。

## 运行与抓包

```bash
# 终端 1：抓包
sudo tcpdump -i lo0 -nn -A port 8084
# 或 Wireshark 选 lo0，display filter: websocket
# 只看握手: http.request.uri == "/ws"
# 只看 Ping/Pong: websocket.ping == 1 或 websocket.opcode == 9

# 终端 2：go run ./websocket/server
# 终端 3：go run ./websocket/client
```

## 你应该看到什么

时间轴（Wireshark 视角）：

```text
No.  Protocol   Info
1-3  TCP        57412 → 8084 [SYN] / [SYN, ACK] / [ACK]     ← 照常的 TCP 握手
4    HTTP       GET /ws HTTP/1.1                            ← Upgrade 请求
     展开可见:  Upgrade: websocket
               Connection: Upgrade
               Sec-WebSocket-Key: ...
               Sec-WebSocket-Version: 13
5    HTTP       HTTP/1.1 101 Switching Protocols
     展开可见:  Upgrade: websocket
               Sec-WebSocket-Accept: s3pPLMBiTxaQ9kYGzzhZRbK+xOo=
6    WebSocket  Masked, client-to-server, opcode: 1 (Text), "hello ws"
7    WebSocket  server-to-client, opcode: 1 (Text), "echo: hello ws"
8    WebSocket  Masked, opcode: 9 (Ping), "keepalive"       ← client 主动心跳
9    WebSocket  opcode: 10 (Pong), "keepalive"              ← server 自动应答
10   WebSocket  Masked, opcode: 8 (Close), status: 1000     ← 协议层告别
11+  TCP        FIN / ACK / FIN / ACK                       ← 然后才是 TCP 挥手
```

实测的客户端与服务端输出：

```text
# client
message: type=1 "echo: hello ws"
pong received: "keepalive"

# server
ws connected: [::1]:57412
frame: type=1 len=8 "hello ws"
ws closed: websocket: close 1000 (normal): bye
```

## 用 curl 手工复现 101

WebSocket 握手只是个 HTTP 请求，curl 就能发（`Sec-WebSocket-Key` 用 RFC 6455 的示例值）：

```bash
curl -si -N \
  -H 'Connection: Upgrade' -H 'Upgrade: websocket' \
  -H 'Sec-WebSocket-Version: 13' \
  -H 'Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==' \
  http://localhost:8084/ws
```

实测响应：

```text
HTTP/1.1 101 Switching Protocols
Upgrade: websocket
Connection: Upgrade
Sec-WebSocket-Accept: s3pPLMBiTxaQ9kYGzzhZRbK+xOo=
```

`Sec-WebSocket-Accept` 可以自己验证：`base64(sha1("dGhlIHNhbXBsZSBub25jZQ==" + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))` 正是 `s3pPLMBiTxaQ9kYGzzhZRbK+xOo=`。curl 会一直挂着等数据（101 之后它不再解析），Ctrl-C 退出即可。

## 帧的掩码在哪看

展开 Wireshark 里 client → server 的帧：

```text
WebSocket
    Fin: True
    Mask: True                    ← 客户端帧必须掩码
    Payload length: 8
    Masking-Key: 4a 92 f3 0b      ← 每帧随机
    Payload: "hello ws"           ← Wireshark 已用 mask 还原
```

服务端 → 客户端的帧 `Mask: False`。这个不对称是 RFC 6455 硬性规定（防代理缓存投毒）。

## 对照记忆

- **101 之前**：全部是标准 HTTP，`http` 过滤器能看到；这意味着 WebSocket 可以复用 HTTP 的端口、代理、认证（Cookie 就在握手请求里）。
- **101 之后**：Wireshark 识别为 `WebSocket` 协议层，HTTP 层消失——但 TCP 层一切照旧（同样的四元组、同样的 SEQ/ACK 推进）。"协议切换"切换的只是**应用层对字节流的解释方式**。
- **关闭的层次**：WebSocket Close 帧（应用层）→ TCP FIN 挥手（传输层）。直接断 TCP 而不发 Close 帧，对端会视为异常断连（浏览器触发 onclose + 重连逻辑）。
