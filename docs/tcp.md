# TCP 概念文档

> 配套实验：`tcp/echo`，抓包现场见 [packet/tcp-handshake.md](../packet/tcp-handshake.md)、[packet/tcp-close.md](../packet/tcp-close.md)、[packet/tcp-seq-ack.md](../packet/tcp-seq-ack.md)、[packet/tcp-byte-stream.md](../packet/tcp-byte-stream.md)。

## TCP 是什么、在哪一层

TCP（Transmission Control Protocol）是**传输层**的面向连接、可靠、字节流协议。它在 IP（网络层，只管把包送到某个 IP 地址、不保证到达、不保证顺序）之上，为应用层补上了：

- **连接**：通信前先握手建立状态
- **可靠**：不丢、不重、不乱序（靠 SEQ/ACK + 重传 + 校验）
- **有序**：字节按发送顺序到达
- **字节流**：像水管，只有"字节流进去流出来"，没有"消息"概念

一条 TCP 连接由四元组唯一标识：`(源IP, 源端口, 目的IP, 目的端口)`。端口就是同一台机器上区分不同进程用的 16 位编号。

## TCP 头部（每个报文都有，20~60 字节）

```text
 0                   1                   2                   3
 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
+-------------------------------+-------------------------------+
|          Source Port          |        Destination Port       |
+-------------------------------+-------------------------------+
|                     Sequence Number (SEQ)                     |
+-------------------------------+-------------------------------+
|                 Acknowledgment Number (ACK)                   |
+-----+-+-------+---------------+-------------------------------+
|HdrLen|R| |U|A|P|R|S| |  Window Size                                 |
| (4b) | | |C|R|S|H|T| |                                             |
+------+ | |E|K|H|M|N| +---------------------------------------------+
|       Checksum                 |         Urgent Pointer       |
+--------------------------------+-------------------------------+
|                      Options (可选, 0~40 字节)                  |
+----------------------------------------------------------------+
```

关键分组理解：

- **端口（16bit × 2）**：进程间寻址。IP 负责找到机器，端口负责找到机器上的 socket。
- **SEQ（32bit）**：本报文第一个数据字节在整个字节流中的位置。
- **ACK（32bit）**：期望对方下发的序号，即"这之前的都收到了"（累计确认）。
- **Flags（SYN/ACK/FIN/RST/PSH/URG）**：控制位。SYN 建连、FIN 关闭、RST 异常重置。
- **Window（16bit）**：接收窗口，流量控制的窗口大小通告。
- **Options**：MSS、窗口缩放、SACK、时间戳等。

## 三次握手

完整抓包分析见 [packet/tcp-handshake.md](../packet/tcp-handshake.md)，这里只留结论：

```text
Client                          Server
  | ---- SYN, seq=x ----------->  |   Client: SYN_SENT
  | <--- SYN+ACK, seq=y, ack=x+1 -|   Server: SYN_RCVD → 分配连接资源
  | ---- ACK, ack=y+1 ----------> |   双方: ESTABLISHED
```

- SYN 报文不带数据但占一个序号；seq 的初始值 ISN 是随机的（防历史连接串扰、防伪造）。
- **为什么三次**：双方都要"通告自己 ISN + 确认对方 ISN"，第 1、2 条完成前两件事，第 3 条是服务端的收据；两次握手下服务端无法确认自己的 ISN 被收到，且会被历史重复 SYN 欺骗建连。
- **谁做的握手**：内核。`net.Dial` → `connect(2)`，阻塞到 ESTABLISHED 才返回；`net.Listen` 之后内核自动应答 SYN，`Accept` 只是从全连接队列取结果。Go 代码不碰任何 TCP 头。

## 四次挥手

抓包分析见 [packet/tcp-close.md](../packet/tcp-close.md)。结论：

```text
主动方                            被动方
  | ---- FIN, seq=u ------------> |   主动方: FIN_WAIT_1
  | <--- ACK, ack=u+1 ----------  |   主动方: FIN_WAIT_2, 被动方: CLOSE_WAIT
  |   （被动方还可以继续发数据）     |
  | <--- FIN, seq=w, ack=u+1 ---  |   被动方: LAST_ACK
  | ---- ACK, ack=w+1 ----------> |   主动方: TIME_WAIT(2MSL) → CLOSED
  |                               |   被动方: 收到 ACK → CLOSED
```

- **为什么四次**：TCP 全双工，两个方向的数据流各自独立关闭。FIN 的语义是"我的数据发完了"，只关一个方向，所以每个方向各要 FIN + ACK 一对。
- **为什么 TIME_WAIT 是主动关闭方**：它要等 2MSL（Linux 60s，macOS 15~30s）——一是保证最后一个 ACK 若丢失，对方重传 FIN 时还能应答；二是让本连接的旧报文在网络中自然死亡，避免污染相同四元组的新连接。
- **CLOSE_WAIT 出现在被动关闭方**：收到 FIN、回了 ACK，但**应用程序还没调用 Close()**。大量 CLOSE_WAIT 几乎总是"代码忘了 close"的 bug 信号。

## SEQ / ACK：可靠与有序的机制

抓包分析见 [packet/tcp-seq-ack.md](../packet/tcp-seq-ack.md)。结论：

- SEQ 表示"本报文数据从字节流哪个位置开始"；ACK 表示"我期望你下一个字节是第几个"。
- ACK 是**累计确认**：`ack=105` 意味着 105 之前的全部收到。中间丢了一段，后面的到了也只能重复确认已收到的最大连续序号（触发快速重传：3 个重复 ACK）。
- 发送方在定时器内收不到确认就重传（超时重传，RTO 随 RTT 动态调整）。
- 接收方用序号排序、去重——这就是"可靠、有序"的全部机制来源。

## 字节流：TCP 没有消息边界

抓包分析见 [packet/tcp-byte-stream.md](../packet/tcp-byte-stream.md)。结论：

- TCP 只保证字节顺序正确，**不保留 Write 的次数和大小**。一次 Write 可能被拆成多个段（半包），多次 Write 可能被合并进一个段（粘包）。
- 所以 `conn.Read(buf)` 返回的 n 与对方 Write 的次数没有任何对应关系；甚至一次大 Write 返回的 n 也可能小于你的 buf 大小。
- 消息边界必须由应用层自己设计：**定长 / 分隔符 / length-prefix**。HTTP 用 header 声明 Content-Length 或 chunked 编码，WebSocket 用帧头里的 length 字段，都是 length-prefix 思路。

## 流量控制与拥塞控制（概念）

- **流量控制**（端到端）：接收方通过头部 Window 字段告诉发送方"我还能收多少"，防止快发送方淹没慢接收方。窗口为 0 时发送方暂停，定期发窗口探测。
- **拥塞控制**（对网络）：发送方维护拥塞窗口 cwnd，慢启动指数增长、到阈值后线性加性增（拥塞避免）、丢包后乘性减（快速重传/快速恢复或超时回退）。实际发送量 = min(rwnd, cwnd)。
- 两者都是内核协议栈行为，Go 代码不可见、不可配置（只能间接通过 SetReadBuffer/SetWriteBuffer 影响缓冲区）。
