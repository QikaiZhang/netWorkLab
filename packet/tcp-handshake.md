# 实验：亲眼看到 TCP 三次握手

对应实验：`tcp/echo`（commit `feat(tcp): implement tcp echo server and client`）。

`net.Dial("tcp", "localhost:9000")` 这一行返回成功之前，内核已经替你完成了三次握手。本实验的任务就是把这三条报文抓出来看。

## 运行与抓包

三个终端依次执行（抓包必须先启动，否则握手发生在抓包之前）：

```bash
# 终端 1：抓包（或打开 Wireshark 选 Loopback: lo0，display filter: tcp.port == 9000）
sudo tcpdump -i lo0 -nn port 9000

# 终端 2：服务端
go run ./tcp/echo/server

# 终端 3：客户端
go run ./tcp/echo/client
```

## 你应该看到什么

tcpdump 输出（seq 数值每次不同，因为 ISN 是随机的）：

```text
12:00:00.000001 IP 127.0.0.1.62987 > 127.0.0.1.9000: Flags [S],  seq 2816874370, win 65535, options [mss 65495,nop,wscale 6,sackOK,TS val ... ecr 0], length 0
12:00:00.000003 IP 127.0.0.1.9000 > 127.0.0.1.62987: Flags [S.], seq 1332938483, ack 2816874371, win 65483, options [mss 65495,nop,wscale 6,sackOK,TS val ... ecr 2816874371], length 0
12:00:00.000004 IP 127.0.0.1.62987 > 127.0.0.1.9000: Flags [.],  ack 1332938484, win ..., length 0
```

在 Wireshark 里，这三条报文的 Info 列分别是：

```text
62987 → 9000    SYN        Seq=0 Win=65535 Len=0 MSS=65495 WS=64 SACK_PERM
9000 → 62987    SYN, ACK   Seq=0 Ack=1 Win=65483 Len=0 MSS=65495 WS=64 SACK_PERM
62987 → 9000    ACK        Seq=1 Ack=1 Win=... Len=0
```

Wireshark 显示的 `Seq=0 / Ack=1` 是**相对序号**（相对第一个 SYN 的 ISN），所以看起来总是 0 和 1；tcpdump 显示的才是真实随机 ISN。抓包时 MAC 上走的是回环接口 `lo0`，不经过物理网卡，但报文格式与真实网络完全一致。

## 字段解释

| 字段 | 第一条 SYN | 第二条 SYN+ACK | 第三条 ACK |
|---|---|---|---|
| Flags | `S`（SYN=1） | `S` + `.`（SYN=1, ACK=1） | `.`（ACK=1） |
| Sequence Number | 客户端随机 ISN，如 2816874370 | 服务端随机 ISN，如 1332938483 | 客户端 ISN+1 |
| Acknowledgment Number | 无意义（0） | 客户端 ISN+1（=2816874371） | 服务端 ISN+1（=1332938484） |
| Length | 0（不带数据） | 0 | 0 |
| 常见 options | MSS、WS、SACK、TS | 同左 | 同左 |

读法：

- **SYN 报文的 seq = 我方初始序号 ISN**。之后我方发送的第 1 个数据字节序号是 ISN+1（SYN 本身按 1 个序号计）。
- **ACK 报文的 ack = 我期望你下一个字节的序号 = 对方 ISN + 1**。这等于告诉对方"你的 ISN 我收到了"。
- SYN 和 FIN 即使不带数据也各占一个序号，所以挥手时序号会 +1。
- options 里的 **MSS**（最大段长）双方各自声明：我这个接口上，单条 TCP 报文的数据部分最多别超过这个数，对方取较小值使用。

## 为什么是三次，两次不行

握手的本质：**双方各自要完成两件事——把自己 ISN 告诉对方 + 确认收到对方 ISN。**

```text
1. C → S: SYN, seq=x        （S 得知 C 的 ISN=x）
2. S → C: SYN+ACK, seq=y, ack=x+1   （C 得知 S 的 ISN=y，且 S 确认了 x）
3. C → S: ACK, ack=y+1      （S 确认了自己的 y 被收到）
```

两次的问题在第 2 条之后结束：服务端无法确认**客户端收到了自己的 ISN**。如果这条 SYN+ACK 在网络中丢了，客户端会认为连接没建好，而服务端已经单方面认为连接可用，状态不一致。第三次 ACK 就是给服务端的收据。

另一个经典理由：防止历史重复连接。客户端很早以前发出的旧 SYN（网络中滞留）晚到了服务端，若两次握手即可建连，服务端会为一个早已废弃的连接分配资源；三次握手下客户端不会对旧连接的 SYN+ACK 发 ACK，服务端等不到第三次握手就放弃。

## TCP 状态变化

```text
客户端:  CLOSED → SYN_SENT ──(收到 SYN+ACK)──→ ESTABLISHED
服务端:  CLOSED → LISTEN ──(收到 SYN)──→ SYN_RCVD ──(收到 ACK)──→ ESTABLISHED
```

握手期间连接既不在 `Accept()` 的已就绪队列，也不归你的 Go 代码管。Linux 内核用两个队列管理：半连接队列（收到 SYN，SYN_RCVD 状态）和全连接队列（完成握手，等 `Accept()` 取走）；`net.Listen` 的 backlog 参数控制的就是全连接队列长度。**半连接队列被塞满是 SYN Flood 攻击的目标**，内核有 syncookies 应对。

## Go 代码与协议的关系

- `net.Listen("tcp", ":9000")` → 内核 `socket()` + `bind()` + `listen()`。此后内核开始替你应答 SYN（发 SYN+ACK），这一步**不需要你的代码参与**。
- `net.Dial("tcp", "localhost:9000")` → 内核 `connect()`。三次握手由内核协议栈完成，`Dial` 阻塞直到收到对端的 SYN+ACK 并发出 ACK（ESTABLISHED）后才返回。
- `ln.Accept()` → 从全连接队列取出一条已完成握手的连接。

也就是说：**Go 程序从不构造 IP 头和 TCP 头，从不收发 SYN/ACK**。这些全是内核协议栈的工作。你写的只是 socket 这一层的系统调用，这就是"传输层由操作系统提供"的含义。

## 验证"握手发生在 Dial 返回前"的一个小实验

故意连一个不存在的端口：

```bash
go run - <<'EOF'
package main
import ("fmt";"net";"time")
func main() {
    start := time.Now()
    _, err := net.DialTimeout("tcp", "localhost:9999", time.Second)
    fmt.Println(time.Since(start), err)
}
EOF
```

会立刻得到 `connection refused`——这是对端内核回了 **RST**（端口没人监听）。注意：能收到 RST 说明网络层是通的，"拒绝"这个动作本身也是 TCP 报文。
