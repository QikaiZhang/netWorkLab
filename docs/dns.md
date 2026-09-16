# DNS 概念文档

> 配套实验：`dns/lookup`、`dns/rawquery`，抓包现场见 [packet/dns.md](../packet/dns.md)。

## DNS 是什么、在哪一层

DNS 是**应用层**协议（尽管它服务于所有其他协议）。它解决的问题很纯粹：人类记域名（`example.com`），路由需要 IP（`104.20.23.154`），DNS 负责翻译。

```text
域名 --DNS--> IP --TCP/UDP--> 通信
```

它不在 TCP/IP 分层里占一层"传输"，就是一个跑在 UDP 53（偶尔 TCP 53）上的查询-响应服务。

## 域名的层级与解析路径

`www.example.com` 从右往左逐级缩小：

```text
.  (根，全球 13 组根服务器)
└── com.  (顶级域 TLD)
    └── example.com.  (权威服务器由域名所有者指定)
        └── www.example.com.
```

一次完整的递归解析（你的机器 → 本地 DNS → 根 → TLD → 权威）：

```text
stub(你的机器)          本地DNS(运营商/1.1.1.1)      根/TLD/权威
    |  www.example.com=A?  |                          |
    |--------------------->| （有缓存就直接回答）        |
    |                      |-- .com 在哪? ------------>|
    |                      |<-- TLD 服务器地址 ---------|
    |                      |-- example.com 的权威在哪? ->|
    |                      |<-- 权威服务器地址 ----------|
    |                      |-- www.example.com=A? ----->| 权威
    |<-- 104.20.23.154 ----|<-- 104.20.23.154 ----------|
```

本地 DNS 替你跑完整个迭代查询并缓存结果。**通常你要写代码的只是第一步**——`net.LookupHost` 连它都封装了。

## 为什么用 UDP

- 查询/响应都是**一次一来回的小报文**（几十到几百字节），无需建连——UDP 0 次握手，一个包出去一个包回来。
- TCP 的话每个查询都要握手+挥手 3 个来回的额外开销，对高频、海量的解析太浪费。
- 可靠性够了：超时没回应就重发（应用层重传，见 `udp/retry` 实验）；查询本身幂等。
- **何时用 TCP**：响应超过 512 字节（传统 UDP 上限，会设置 TC 截断标志，客户端改用 TCP 重查）；DNS 区域传送（zone transfer）；现代 DNS over TLS (DoT)/DNS over HTTPS (DoH) 也走 TCP。

## 报文格式（rawquery 实验手工拼的就是它）

```text
Header (12 字节，固定)
  ID(16bit)        查询标识，响应原样带回 → 匹配问答（无连接所以必须带）
  Flags(16bit)     QR(0问1答) | Opcode | AA | TC | RD(递归请求) | RA | RCODE(0=成功)
  QDCOUNT/ANCOUNT/NSCOUNT/ARCOUNT   各区条数

Question (每次查询 1 条)
  QNAME   域名编码：每个 label 前面放长度字节，0 结尾
          3www7example3com0
  QTYPE   1=A(IPv4) | 28=AAAA(IPv6) | 5=CNAME | 15=MX | 16=TXT ...
  QCLASS  1=IN(Internet)

Answer (0~n 条)
  NAME(可压缩成 2 字节指针 0xC0xx) | TYPE | CLASS | TTL | RDLENGTH | RDATA
  A 记录的 RDATA 就是 4 字节 IPv4
```

响应里的**名字压缩指针**是 DNS 报文最有意思的设计：`www.example.com` 在 Question 里出现过一次，Answer 的 NAME 就不重复存，用 `0xC00C` 指向偏移 12 处——这就是 `dns/rawquery` 里 `readName` 处理指针的原因。

## 记录类型

| 类型 | 含义 | 例子 |
|---|---|---|
| A | 域名 → IPv4 | example.com → 104.20.23.154 |
| AAAA | 域名 → IPv6 | example.com → 2606:4700:10::6814:179a |
| CNAME | 别名 → 另一个域名（不能再指向 IP） | www.baidu.com → www.a.shifen.com |
| MX | 邮件服务器 | @ → mail.example.com |
| NS | 区域的权威服务器 | example.com → ns1.example.com |
| TXT | 任意文本（SPF/域验证） | "v=spf1 ..." |

实测 `go run ./dns/rawquery www.baidu.com 8.8.8.8` 的输出正好演示了 CNAME 链：

```text
answer: type=5 ttl=155 data=www.baidu.com      ← CNAME
answer: type=5 ttl=19  data=www.a.shifen.com   ← CNAME
answer: type=1 ttl=13  data=103.235.46.115     ← 最终 A 记录
answer: type=1 ttl=13  data=103.235.46.102
```

## DNS 缓存与 TTL

- 每条记录带 **TTL**（秒），多级缓存：浏览器缓存 → 操作系统缓存（macOS 的 mDNSResponder）→ 本地 DNS 服务器 → 权威。
- 所以改 DNS 记录后"全球生效要等 TTL"——旧缓存到期才被重新查询。调试时 `sudo dscacheutil -flushcache`（macOS）只清本机缓存。
- 实验里 `ttl=13` 说明该记录缓存策略很短（CDN 常见，方便快速调度流量）。

## Go 代码与协议的关系

- `net.LookupHost`：调用**操作系统解析器**（macOS 走 mDNSResponder、Linux 走 nsswitch/resolv.conf），缓存、TCP 回退、搜索域全都替你处理。
- `net.Dial("udp", "1.1.1.1:53")`：任何会写 UDP 的代码都能发 DNS 查询——`dns/rawquery` 的 100 行证明了 DNS 没有魔法，只是约定好的二进制格式。
- 生产中手写 DNS 不现实（名字压缩、EDNS0、DNSSEC…），需要时用 `github.com/miekg/dns` 这类成熟库；理解报文格式是为了读懂抓包和面试。
