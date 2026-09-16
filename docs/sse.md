# SSE（Server-Sent Events）概念文档

> 配套实验：`sse/server`，抓包现场见 [packet/sse.md](../packet/sse.md)。

## 一句话结论

```text
SSE 不是 TCP，不是 WebSocket，不是新的传输层协议。
SSE 是建立在 HTTP 之上的、Server → Client 的单向事件流机制：
一个"永不结束"的 HTTP 响应体，按约定格式切分事件。
```

## 8 个关键问题

### 1. SSE 为什么可以持续推送？

因为 HTTP 响应体本来就是**长度不限的字节流**：只要服务器不写完 `Content-Length` 声明的字节数（或干脆用 chunked / 不结束），HTTP 响应就没有"完成"一说。SSE 服务器只是**故意不结束响应**，每隔一段时间往响应体里写一小段格式化文本并 flush。

### 2. SSE 是否建立了新的传输层协议？

没有。`curl http://localhost:8083/events` 与请求普通网页完全一样：一次 TCP 三次握手 + 一个 `GET /events` + 一个 `HTTP/1.1 200 OK`。没有任何 Upgrade、没有新协议握手。抓包里你能看到每一个 `data: message N` 都是普通 HTTP chunked chunk，跑在同一个 TCP 连接上。

### 3. SSE 与普通 HTTP 响应有什么区别？

只有两点约定：

- 响应头 `Content-Type: text/event-stream`（告诉客户端"这是事件流，请按事件格式解析"）。
- 响应体格式：事件之间用**空行**分隔，每个事件由 `data:` / `id:` / `event:` / `retry:` 等字段行组成：

```text
retry: 3000
                      ← 空行结束一个"事件"
id: 1
data: message 1
                      ← 空行 = 事件边界
id: 2
data: message 2
```

它就是 HTTP，只是响应体"无限长"。

### 4. SSE 与 WebSocket 有什么区别？

| | SSE | WebSocket |
|---|---|---|
| 底层 | 普通 HTTP 响应 | HTTP Upgrade 升级成独立协议 |
| 方向 | **单向**：Server → Client | 双全工 |
| 数据格式 | 文本（UTF-8） | 文本或二进制帧 |
| 断线重连 | 浏览器**自动**重连 + `Last-Event-ID` 续传 | 应用自己实现 |
| 依赖 | 只要 HTTP 服务器/代理都支持 | 中间代理需支持 Upgrade |
| 典型场景 | 推送通知、行情、进度条 | 聊天、协同编辑、游戏 |

需要客户端→服务端高频通信时用 WebSocket；只是服务端推送时 SSE 简单得多。

### 5. SSE 为什么天然主要是 Server → Client？

因为 HTTP 请求-响应模型里，**客户端只能"问"，服务端只能"答"**。SSE 的"通道"就是一个超长的"答"，服务端可以一直"答"下去，但客户端在这次响应里没有说话的机制——想再说话就发新请求。

### 6. 浏览器为什么可以用 EventSource？

`EventSource` 是浏览器内置的 JavaScript API，它做的事：发起普通 `GET` 请求 → 校验响应 `Content-Type` 是 `text/event-stream` → 持续解析响应体 → 每遇到空行分隔的完整事件就触发 `onmessage`（或 `addEventListener('事件名')`）。没有特殊网络能力，全是文本解析。实验里 `sse/server` 的首页就带一个 EventSource 演示页。

### 7. SSE 断线重连是怎么回事？

浏览器 `EventSource` **内置自动重连**：连接断开后等待 `retry:` 字段指定的毫秒数（未指定则约 3 秒），重新发 `GET /events`，并带上请求头 `Last-Event-ID: <最后收到的事件id>`。服务器据此补发漏掉的事件——这就是 SSE 事件里 `id:` 字段存在的意义。这一切都是**HTTP 层的普通请求与响应头**，没有任何新协议机制。

### 8. `text/event-stream` 是什么？

一个普通的 MIME 类型（和 `text/html`、`application/json` 并列）。HTTP 里 Content-Type 的作用就是"这个 Body 该按什么格式解释"；`text/event-stream` 的含义是"按事件流格式、流式地解释我"。它不改变传输方式，只改变客户端解析 Body 的方式。

## Go 代码与协议的关系

- `w.Header().Set("Content-Type", "text/event-stream")` → 只是写一行响应头。
- `fmt.Fprintf(w, "id: %d\ndata: message %d\n\n", ...)` → 只是往响应体写字节，空行是事件边界。
- `flusher.Flush()` → 触发 chunked 编码立即把已写字节发出去；没有它，标准库会等缓冲区攒满，客户端就"感觉不到推送"了。**响应没有 Content-Length 时，Go 自动使用 chunked 编码**（curl 实测响应头里的 `Transfer-Encoding: chunked`）。
- 循环永不退出 → 响应永不结束 → 这就是"长连接推送"的全部秘密。
