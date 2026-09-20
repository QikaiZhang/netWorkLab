# 实验：TCP 没有消息边界（粘包 / 半包）

对应实验：`tcp/stream`。Client 用**两种节奏**发送同样的 `hello` + `world` 两个词，Server 打印每一次 `Read()` 实际读到的内容。

## 运行

```bash
# 终端 1
go run ./tcp/stream/server
# 终端 2
go run ./tcp/stream/client
```

## 真实运行结果

本仓库实测输出（每次运行结论一致，具体合并情况可能因机器负载略有差异）：

```text
byte-stream server listening on :9001
[conn 1] connected from [::1]:65494
[conn 1] Read() -> 10 bytes: "helloworld"
[conn 1] closed (EOF)
[conn 2] connected from [::1]:65495
[conn 2] Read() -> 5 bytes: "hello"
[conn 2] Read() -> 5 bytes: "world"
[conn 2] closed (EOF)
```

- **conn 1**（两次 Write 背靠背）：Client 调了两次 `Write`（5B + 5B），Server 只调了**一次** `Read` 就拿到 10 字节 `"helloworld"`。两次写入被合并——**粘包**。
- **conn 2**（两次 Write 隔 1 秒）：Server 两次 `Read` 各得 5 字节。不是因为 TCP"尊重"了边界，只是因为数据到达间隔 1 秒，Server 先到了先读。

同一对 Write 调用，两种 Read 结果。结论只有一条：

> **TCP 是字节流：它保证字节一个不丢、顺序不变，但不保留"写"的次数与大小。`一次 Write = 对方一次 Read` 这个假设永远是错的。**

反过来也存在：一次 `Write` 64KB 会被拆成多个 MSS 大小的段，Server 需要多次 Read 才能拿全——**半包**。你甚至可能一次 Read 只拿到半个消息（内核缓冲区里当时有多少就给多少）。

顺带说明：Go 的 TCP 连接默认已设置 `TCP_NODELAY`（禁用 Nagle 算法），所以这里的合并不是 Nagle 攒数据造成的，而是回环网络太快——数据在 Server 的 goroutine 被调度到 Read 之前就都进了接收缓冲区。**无论网络多快多慢，流式语义都成立**，不能靠"发快点/等一等"来解决。

## 抓包视角

```bash
sudo tcpdump -i lo0 -nn port 9001
```

conn 1 通常只能看到一条 `length 10` 的数据报文（两次 Write 的数据进了同一段）；conn 2 能看到两条相隔 1 秒的 `length 5` 报文。**在线路上本来就没有"消息"这回事，只有字节流的片段（segment）。**

## 为什么必须由应用层设计消息边界

应用真正想要的单位是"消息"（一条命令、一个请求），而 TCP 给的是"字节流"。要在流上切出消息，只有三种基础方案：

| 方案 | 格式示例 | 优点 | 缺点 |
|---|---|---|---|
| **固定长度** | 每条固定 16 字节 | 解析最简单 | 浪费带宽 / 长消息装不下 |
| **分隔符** | `hello\nworld\n` | 文本协议直观（Redis、SMTP） | 内容本身不能含分隔符，需转义 |
| **长度前缀（length-prefix）** | `[4字节长度][数据]` | 二进制友好、无歧义、解析 O(1) | 要处理"长度字段跨段到达" |

业界几乎都用第三种。前面 echo 里随手写的 `bufio.Scanner` 按 `\n` 切行，其实就是"分隔符方案"。

## 现成的协议是怎么解决的

- **HTTP/1.1**：请求行 + 头部，以 `\r\n\r\n` 结束（分隔符）；Body 长度由 `Content-Length` 明确给出（length-prefix 变体），或 `Transfer-Encoding: chunked` 按块声明。HTTP/2 则把内容切成带长度的帧（frame）。
- **WebSocket**：每个帧头自带 7/16/64 bit 的 payload 长度字段。
- **gRPC**：HTTP/2 帧之上，每条消息前还有 5 字节 gRPC 长度前缀（1 字节压缩标志 + 4 字节大端长度）。

也就是说：**所有建立在 TCP 上的应用层协议，第一件事就是定义消息边界。** 这也是 Phase 3 手写最小 HTTP parser 的入口。

## 一个正确的接收模板

无论哪种方案，服务端读取都要循环读、攒够再解析：

```go
reader := bufio.NewReader(conn)
for {
    // length-prefix 例：先读满 4 字节长度，再读满该长度的 body
    var lenBuf [4]byte
    if _, err := io.ReadFull(reader, lenBuf[:]); err != nil {
        return // EOF 或错误
    }
    body := make([]byte, binary.BigEndian.Uint32(lenBuf[:]))
    if _, err := io.ReadFull(reader, body); err != nil {
        return
    }
    handle(body)
}
```

`io.ReadFull` 的意义就是对抗"半包"：Read 少了就继续读，直到拿满声明长度。
