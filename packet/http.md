# 实验：抓 HTTP 请求 / 响应的原始字节

对应实验：`http/server`（net/http）与 `http/minparser`（裸 TCP）。两个实验都能抓，建议先抓标准库版，再用 minparser 对照。

## 运行与抓包

```bash
# 终端 1：抓包（先启动）。-A 直接打印 ASCII，肉眼读 HTTP 最方便
sudo tcpdump -i lo0 -nn -A port 8081

# 或 Wireshark 选 lo0，display filter: http，抓完右键 Follow → TCP Stream

# 终端 2：go run ./http/server
# 终端 3：
curl -sv http://localhost:8081/hello
curl -sv -X POST -H 'Content-Type: application/json' -d '{"name":"kian"}' http://localhost:8081/users
```

## 你应该看到什么

`curl -v` 的真实输出（它把发出的请求和收到的响应都用 `>` `<` 标出）：

```text
> GET /hello HTTP/1.1
> Host: localhost:8081
> User-Agent: curl/8.7.1
> Accept: */*
>
< HTTP/1.1 200 OK
< Date: Wed, 16 Sep 2026 16:39:11 GMT
< Content-Length: 44
< Content-Type: text/plain; charset=utf-8
<
hello, this is http over tcp (proto=HTTP/1.1)
```

POST 时多出请求头与 Body：

```text
> POST /users HTTP/1.1
> Host: localhost:8081
> Content-Type: application/json
> Content-Length: 16
>
{"name":"kian"}
```

注意请求头里**没有** `Connection: close`——HTTP/1.1 默认 keep-alive，连续 curl 两个 URL 时第二条会显示 `Re-using existing connection`（复用同一条 TCP 连接）。

## 在 Wireshark 里对照 TCP 分层

同一个包自上而下展开：

```text
Ethernet II                  ← 本机回环没有，但真实网卡上有
Internet Protocol Version 4  ← Src: 127.0.0.1, Dst: 127.0.0.1
Transmission Control Protocol ← Src Port: 51156, Dst Port: 8081, Seq=1, Len=57
Hypertext Transfer Protocol   ← GET /hello HTTP/1.1\r\n ...
```

点击 HTTP 层，Wireshark 按"请求行 / Header / Body"分块高亮——**Wireshark 能这么做，恰恰说明 HTTP 是纯文本、有固定结构的应用层消息**。GET 请求 Len=57 的那段 TCP 数据，内容就是 curl 显示的那几十个字节。

## minparser 实验：HTTP 就是字节

`go run ./http/minparser/server` + `go run ./http/minparser/client` 的实测输出——客户端手工拼 HTTP 字节、服务端手工解析，两端都完全脱离 net/http：

```text
--- request bytes ---
POST /echo HTTP/1.1
Host: localhost:8082
Content-Type: text/plain
Content-Length: 5

hello
--- response bytes (122) ---
HTTP/1.1 200 OK
Content-Type: text/plain
Content-Length: 38
Connection: close

you sent POST /echo with 5 body bytes
```

服务端解析逻辑（也是所有 HTTP 服务器的骨架）：

```text
读一行 → 请求行（方法/路径/版本）
逐行读 → 直到空行（头部区）
按 Content-Length 读满 N 字节 → Body
```

其中 `Content-Length: 5` 后面"不多读一个字节、不少读一个字节"的行为，正是对 TCP 无消息边界的回应（见 [docs/http.md](../docs/http.md#content-lengthhttp-对消息边界的回答)）。这个 60 行的 parser 能正常响应 curl，证明：**HTTP 没有任何魔法，只是大家约定好的文本格式跑在 TCP 上。**

## 常见问题

- **为什么 tcpdump 里有时看不到 `http` 层内容**：HTTPS 流量（443）在 tcpdump 里全是密文；本实验用明文 8081 所以可见。Wireshark 的 `http` 过滤器只匹配明文 HTTP。
- **抓包里看到 TCP 层很多 Len=0 的包**：那是纯 ACK（确认对方的段），HTTP 消息本身在 Len>0 的段里。
- **一个包里出现两个请求**：keep-alive 连接上小请求可能被合并发送（TCP 粘包的正常表现），Wireshark 会拆成两个 HTTP dissector 条目——应用层协议边界由解析器恢复，线路上的 TCP 段不管这些。
