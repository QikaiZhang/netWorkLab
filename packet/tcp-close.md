# 实验：亲眼看到 TCP 四次挥手与 TIME_WAIT

对应实验：`tcp/echo`。客户端发送完数据后 `conn.Close()`，这一行触发四次挥手。

## 运行与抓包

```bash
# 终端 1：抓包（先启动）
sudo tcpdump -i lo0 -nn port 9000
# 或 Wireshark 选 lo0，display filter: tcp.flags.fin == 1

# 终端 2：go run ./tcp/echo/server
# 终端 3：go run ./tcp/echo/client
```

## 你应该看到什么

tcpdump 中最后四条报文（本实验 echo server 在收到 FIN 后立即 Close，所以两边几乎同时关）：

```text
12:00:10.000100 IP 127.0.0.1.64631 > 127.0.0.1.9000: Flags [F.], seq 29, ack 29, win ..., length 0
12:00:10.000101 IP 127.0.0.1.9000 > 127.0.0.1.64631: Flags [.],  ack 30, win ..., length 0
12:00:10.000102 IP 127.0.0.1.9000 > 127.0.0.1.64631: Flags [F.], seq 29, ack 30, win ..., length 0
12:00:10.000103 IP 127.0.0.1.64631 > 127.0.0.1.9000: Flags [.],  ack 31, win ..., length 0
```

Wireshark Info 列依次是 `FIN, ACK` / `ACK` / `FIN, ACK` / `ACK`。注意两个细节：

- FIN 也占一个序号，所以序号比最后一个数据字节大 1（echo 了 28 字节数据 + 换行前序号 1 起，最后 seq=29 起 FIN）。
- 中间两个报文经常被合并：第二、三条是同一端发出的（先 ACK 对方 FIN，再发自己的 FIN），抓包里也可能看到 `FIN, ACK` 一条搞定，这不影响状态机。

## 真实状态观察：TIME_WAIT 在谁身上

客户端是主动关闭方。`go run ./tcp/echo/client` 结束后**立刻**执行 `netstat -an -p tcp | grep 9000`，实测输出（macOS）：

```text
tcp46      0      0  *.9000                 *.*                    LISTEN
tcp6       0      0  ::1.64631              ::1.9000               TIME_WAIT
```

- `64631` 正是客户端日志里打印的本地临时端口（`local addr: [::1]:64631`）——**TIME_WAIT 归主动关闭方**。
- 服务端只有 `LISTEN`：它在收到最后一个 ACK 后已经彻底 CLOSED，四元组资源已释放。
- 这个 TIME_WAIT 会停留约 15~30 秒（macOS 默认 2*MSL=30s，Linux 是 60s），期间这个临时端口不能被复用于新连接（同一四元组）。用 `while true` 循环跑 client，能看到一堆不同临时端口的 TIME_WAIT——这就是高并发短连接客户端会遇到"临时端口耗尽/分不清新旧连接"问题的现场。

## 为什么是四次

TCP 是**全双工**的：两个方向的数据流互相独立。FIN 的语义是"我这边不会再发数据了"（但**还能收**）。所以：

```text
1. A → B: FIN        A 的发送方向关闭
2. B → A: ACK        B 确认（B 自己可能还有数据要发）
3. B → A: FIN        B 的数据也发完了
4. A → B: ACK        A 确认
```

如果 B 收到 FIN 时已经没有数据要发，2、3 可以合并成一条 `FIN, ACK`（Wireshark 里经常看到三条报文完成挥手，就是合并了）。

还有一种"三次挥手都不到"的极端情况：一端直接发 **RST**（连接重置）——崩溃、端口无监听、socket 设置 SO_LINGER=0 等，挥手流程直接跳过。

## CLOSE_WAIT：应用层 bug 的信号灯

被动关闭方的状态流转：`ESTABLISHED → CLOSE_WAIT（收到 FIN 并 ACK 后）→ LAST_ACK（自己 Close 后）→ CLOSED`。

关键：**从 CLOSE_WAIT 到 LAST_ACK 之间由应用程序控制**。如果代码收到 EOF 后没有调用 `Close()`，连接就永远停在 CLOSE_WAIT，服务端 fd 泄漏。线上服务 `netstat` 里大量 CLOSE_WAIT，几乎可以断定某处漏了 close 或 close 前被阻塞。

对照：TIME_WAIT 是主动方的正常状态，量大但会自动消失；CLOSE_WAIT 是被动方的异常滞留，不会自动消失。

## Go 代码与协议的关系

- `conn.Close()` → 内核 `close(fd)`：**先**把发送缓冲区里剩余数据发完，再发 FIN。所以 Close 不会丢你已 Write 的数据。
- `conn.(*net.TCPConn).CloseWrite()` → 只发 FIN 不关读方向，对应"半关闭"：对端可以继续发数据给你（消息尾部标志的场景）。HTTP/1.0 的"读响应到 EOF"就依赖客户端半关闭发请求。
- 读取端视角：对端 Close 后，你这边 `Read` 返回 `io.EOF`——这就是 echo server 日志里 `disconnected: ... (EOF)` 的来源。EOF 是"对端关闭了发送方向"的应用层表现。
