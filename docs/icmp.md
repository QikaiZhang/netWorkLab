# ICMP 概念文档

> 配套实验：`icmp/ping`，抓包现场见 [packet/icmp.md](../packet/icmp.md)。

## ICMP 是什么、在哪一层

ICMP（Internet Control Message Protocol）是 IP 的"控制面"：它**承载的不是用户数据，而是网络自身的控制与差错信息**——"目标不可达"、"超时"、"重定向"、"请求回应"。

```text
ping     = ICMP Echo Request (Type 8) → Echo Reply (Type 0)
         不使用 TCP
         不使用 UDP
```

分层的准确说法：ICMP 与 TCP/UDP 平级，都直接跑在 IP 之上（IP 头的 Protocol 字段：TCP=6，UDP=17，**ICMP=1**）。差别在于 TCP/UDP 有端口、面向进程，**ICMP 没有端口概念**，由内核直接处理并转交。

## ICMP 报文结构（Echo 场景）

```text
IP 头（Protocol=1）
└─ ICMP 头（8 字节）
   Type(1B):   8=Echo Request, 0=Echo Reply
   Code(1B):   0
   Checksum(2B)
   Identifier(2B) ← 匹配请求与响应（类似 TCP 的端口，但由内核/进程自定义）
   Sequence(2B)   ← 第几次 ping
└─ Data（任意字节，回显原样返回）
```

 Identifier + Sequence + Data 三件套一起回显——我们的实验正是靠 `data="HELLO-ICMP"` 确认"这个回复对应我的请求"。

## ICMP 与 IP 的关系

- ICMP 是 IP 的**配套协议**：IP 只管尽力转发，出了问题（不可达、超时、TTL 耗尽）靠 ICMP 反馈。
- 经典应用：**traceroute** 利用 "TTL 减到 0 时路由器回 ICMP Time Exceeded"——逐个递增 TTL，就能画出路径上的每一跳。
- 常见 Type 速查：0/8 Echo、3 Destination Unreachable（子码区分：端口不可达/网络不可达/需要分片…）、11 Time Exceeded、5 Redirect。

## 权限：为什么 ping 代码要"特殊待遇"

raw socket 发 ICMP 需要 root（Linux 上尤其严格）。各平台给"非 root ping"开的通道不同：

- **macOS/BSD**：内核允许非特权用户打开 `SOCK_DGRAM + IPPROTO_ICMP` 的 socket（"非特权 ICMP datagram socket"）。本仓库实验用的就是它，实测无需 sudo。
  - 坑：`x/net/icmp` 的 `ListenPacket("udp4", "0.0.0.0")` 会先 bind，在较新的 macOS 上 `Sendto` 会报 `invalid argument`（实测如此）；直接用 `syscall.Socket` 创建**不绑定**的 datagram ICMP socket 即可。这也是代码里出现 `syscall.Socket` 的原因。
- **Linux**：`SOCK_DGRAM ICMP` 需要 root 或 `net.ipv4.ping_group_range` sysctl 放行；最通用的是 `sudo go run ./icmp/ping` 改用 raw socket。
- 系统自带 `ping` 命令有的平台靠 setuid/权限 entitlement，与你的代码无关。

## Go 代码与协议的关系

- `syscall.Socket(AF_INET, SOCK_DGRAM, IPPROTO_ICMP)`：绕过 net 包，直接向内核申请 ICMP 通道。
- `icmp.Message.Marshal(nil)`：组 ICMP 头 + 计算校验和；`icmp.ParseMessage(1, ...)` 反向解析。**IP 头不需要你构造**——内核负责加 IP 头、路由、ARP；你只负责 ICMP 部分。
- 收到的报文**自带 IP 头**（代码里按 IHL 跳过 20 字节）——因为 ICMP datagram socket 收到的是完整 IP 报文。
- 这与 TCP/UDP 实验形成鲜明对照：TCP/UDP 时你面对的是"流/数据报"，IP 完全不可见；ICMP 时内核只替你做了 IP 层。
