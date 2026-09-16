# HTTP 概念文档

> 配套实验：`http/server`（标准库）、`http/minparser`（基于 TCP 手写解析），抓包现场见 [packet/http.md](../packet/http.md)。

## HTTP 在哪一层

HTTP 是**应用层**协议。它自己不传输任何东西，只是规定了"请求/响应消息长什么样"，然后把这些字节交给 TCP 传输：

```text
HTTP（规定消息格式与语义）
 ↓
TCP（可靠字节流，负责送达）
 ↓
IP（负责寻址与路由）
```

所以"HTTP 服务器"本质是一个 TCP 服务器 + 一套文本解析规则。`http/minparser` 实验专门验证这一点：用 `net.Listen` 手写 60 行，就是一个迷你 HTTP 服务器。

## 请求与响应的结构

HTTP/1.1 的报文是"ASCII 头部 + 可选二进制体"，行尾一律 `\r\n`：

```text
# 请求
POST /users HTTP/1.1\r\n          ← 请求行：方法 路径 版本
Host: localhost:8081\r\n          ← 头部区：Key: Value
Content-Type: application/json\r\n
Content-Length: 16\r\n
\r\n                              ← 空行 = 头部结束
{"name":"kian"}                   ← Body（不一定有）

# 响应
HTTP/1.1 200 OK\r\n               ← 状态行：版本 状态码 原因短语
Content-Type: application/json\r\n
Content-Length: 23\r\n
\r\n
{"id":1,"name":"kian"}
```

规则只有几条：

- **请求行/状态行**：协议版本 + 资源标识 + 结果。
- **Header**：元数据，`Key: Value`。重要的几个：`Host`（一台服务器托管多个站点靠它区分）、`Content-Type`（Body 是什么格式）、`Content-Length`（Body 多长）、`Connection`（连接管理）。
- **空行**：`头部`与`Body`的分界。没有 Content-Length 也没有 chunked 时，读完头部就结束（如多数 GET）。
- **Body**：任意字节。JSON、表单、图片、Protobuf 都行，靠 Content-Type 解释。

## 方法与常见状态码

- 方法语义：`GET` 读（幂等、无 Body）、`POST` 创建/提交、`PUT` 全量替换、`DELETE` 删除、`HEAD` 只要头部。方法只是"约定"，服务器想怎么解释都行，RESTful 是把语义用得一致的实践。
- 状态码：`2xx` 成功、`3xx` 重定向（301 永久/302 临时）、`4xx` 客户端错（404 找不到资源、403 没权限、429 限流）、`5xx` 服务端错（500 异常、502 网关后面的服务挂了、504 网关超时）。

## Content-Length：HTTP 对"消息边界"的回答

TCP 没有消息边界（见 [packet/tcp-byte-stream.md](../packet/tcp-byte-stream.md)）。HTTP 的解决方案：

- 有 Body 时：头部声明 `Content-Length: N`，读方数够 N 字节就知道消息结束——length-prefix 思想的变体（长度在前面，只不过写在文本头部里）。
- 长度未知时：`Transfer-Encoding: chunked`，Body 切成块，每块自带长度，以长度为 0 的块结束。
- 无 Body 时：读完头部即结束。

如果两者都没有、连接也不关，读方只能一直等——这是新手手写 HTTP 客户端最常踩的坑。

## Keep-Alive：一条 TCP 连接跑多个请求

HTTP/1.0 每个请求都新建 TCP 连接（握手 + 挥手成本高）；HTTP/1.1 默认 `Connection: keep-alive`，一条连接上可以串行跑多个请求/响应，由 Content-Length 来区分每条消息的边界——这正是它必须存在的原因之一。

用 curl 验证（同一条连接跑两个请求）：

```bash
curl -sv http://localhost:8081/hello http://localhost:8081/hello 2>&1 | grep -E 'Re-using|Connected'
# 第二个请求会显示: Re-using existing connection with host localhost
```

## HTTP/1.1 → HTTP/2 → HTTP/3

| | HTTP/1.1 | HTTP/2 | HTTP/3 |
|---|---|---|---|
| 传输层 | TCP | TCP | **QUIC（UDP）** |
| 报文格式 | 文本 | 二进制分帧 | 二进制分帧 |
| 并发 | 一条连接一次一个请求（pipelining 不可用） | **多路复用**：一条连接多个 stream 并发 | 同 HTTP/2，且无 TCP 层队头阻塞 |
| 头部 | 每次全量重发 | HPACK 压缩 | QPACK 压缩 |
| 部署 | 明文或 TLS | 实践中几乎都跑在 TLS 上 | 强制加密 |

关键演进逻辑：

- HTTP/2 的**多路复用**解决 HTTP/1.1 的队头阻塞（一条连接上第一个请求没完，第二个只能等），但 TCP 层的队头阻塞仍在——一个段丢了，TCP 必须等它重传回来，所有 stream 的字节都卡住。
- HTTP/3 干脆换掉 TCP：QUIC 在 UDP 上自己实现可靠传输，stream 之间独立重传，彻底消除传输层队头阻塞。
- 版本切换怎么发生：HTTPS 下由 TLS 握手阶段协商（ALPN 扩展带 `h2` / `http/1.1`），明文 HTTP 则靠 `Upgrade: h2c` 或首次响应头 `Alt-Svc` 提示。

## Go 代码与协议的关系

`net/http` 把两层全包了：

- 服务端：`http.ListenAndServe(":8081", mux)` = TCP `Listen` + `Accept` 循环 + 每连接 goroutine + HTTP 解析 + 路由（mux）+ 并发管理。你只写 `func(w, r)`。
- 客户端：`http.Get(url)` = DNS 解析 + `Dial` + 写请求字节 + 读解析响应 + 连接池复用（keep-alive）。
- `r.Body` 是流式的：标准库按 Content-Length/chunked 从 TCP 里读出正确字节数。
- 想看"标准库到底替你做了什么"→ 跑 `http/minparser`，那 60 行是 ListenAndServe 的骨架。
