# 网络编程面试复习（校招版）

> 每题按 **一句话回答 → 原理 → 实验 → 抓包现象 → 面试展开** 组织。引用的实验都可运行，抓包现象都可在本机复现。

---

## 一、TCP

### 1. TCP 是什么？

- **一句话**：传输层的面向连接、可靠、有序的**字节流**协议。
- **原理**：在 IP（不可靠、无连接、按地址投递）之上提供"进程到进程"（IP+端口）的可靠通道。
- **实验**：`tcp/echo`。
- **展开**：说出"字节流"三个字必被追问消息边界（见第 9 题）；说出"连接"必被追问握手挥手；可靠必被追问 SEQ/ACK 与重传。

### 2. TCP 为什么可靠？

- **一句话**：序号 + 累计确认 + 超时/快速重传 + 校验和 + 去重排序，外加滑动窗口做流控。
- **原理**：每个字节都有序号；接收方用 `ACK=下一个期望字节` 累计确认；发送方超时或收到 3 个重复 ACK 就重传；接收方按序号重排、丢弃重复。
- **实验**：`tcp/echo`（观察 EOF 与回显）。
- **抓包**：`packet/tcp-seq-ack.md`——28 字节数据发出后，对端 `Ack=29`（推进量=新收字节数）。
- **展开**：可靠 ≠ 不丢包，而是"丢了能补回来"；校验和只保证"检出损坏"，不负责修复。

### 3. 三次握手过程？

- **一句话**：SYN（带客户端随机 ISN）→ SYN+ACK（服务端 ISN + 确认）→ ACK。
- **原理**：双方各自要"通告自己的初始序号 + 确认对方的初始序号"，共 4 个动作，第 2 步的 SYN 和 ACK 合并，剩 3 条报文。
- **实验**：`tcp/echo`（Dial 返回即握手完成）。
- **抓包**：`packet/tcp-handshake.md`——`Flags [S]`、`Flags [S.]`、`Flags [.]` 三条，seq 随机、ack=对方 seq+1、Len=0。
- **展开**：握手由内核完成，`net.Dial` → `connect(2)`；半连接队列（SYN_RCVD）与全连接队列（backlog）；SYN Flood 与 syncookies。

### 4. 为什么是三次，不是两次/四次？

- **一句话**：两次无法让服务端确认"自己的 ISN 被收到"，且会被滞留的旧 SYN 欺骗建连；四次没必要，因为 SYN+ACK 可以合并。
- **原理**：核心目标是**同步双方初始序号并防止历史连接**。旧 SYN 迟到时，三次握手中的最后一次 ACK 不会发出，服务端不会白建连接。
- **展开**：这是最高频追问。回答套路：先说"要同步的信息有几份"（两个 ISN，各需一问一答），再说"哪两步能合并"（服务端的 SYN+ACK），结论自然是 3；最后补历史连接与状态不一致两个反例。

### 5. 四次挥手过程？为什么是四次？

- **一句话**：FIN → ACK → FIN → ACK，因为全双工的两个方向要**各自独立关闭**。
- **原理**：FIN 语义是"我发完了"，只关一个方向；被动方可能还有数据（CLOSE_WAIT 期间可以继续发）。
- **抓包**：`packet/tcp-close.md`——四条 `F././F./.`（被动方无数据时中间两条会合并成 `FIN,ACK`，看起来像三次）。
- **展开**：Go 的 `CloseWrite()` 可演示半关闭；HTTP/1.0 依赖半关闭读响应到 EOF。

### 6. TIME_WAIT 是什么？谁会有？为什么等 2MSL？

- **一句话**：**主动关闭方**在发出最后一个 ACK 后停留 2MSL（Linux 60s / macOS 15~30s）的状态，作用是保证最后的 ACK 丢失时能重发，并让旧报文自然消亡。
- **实验**：`tcp/echo` 客户端是主动方。
- **实测**：client 退出后 `netstat` 可见其临时端口 `TIME_WAIT`，server 端只剩 LISTEN（`packet/tcp-close.md` 有真实输出）。
- **展开**：TIME_WAIT 过多（高并发短连接客户端）→ 临时端口耗尽；`SO_REUSEADDR` 允许监听端口复用但不动 TIME_WAIT 语义；服务端主动关闭会把 TIME_WAIT 堆在服务端，所以常说"让客户端先关"。

### 7. CLOSE_WAIT 是什么？

- **一句话**：**被动关闭方**收到 FIN 并 ACK 之后、自己还没调用 close 的状态；大量出现 = 代码忘了 close。
- **展开**：与 TIME_WAIT 的对比是面试陷阱题——TIME_WAIT 是协议设计的正常状态、会自动消失；CLOSE_WAIT 是应用 bug 的滞留状态、不会自己消失。排查：找到没 close 的代码路径（通常是没有读循环检测 EOF）。

### 8. SEQ 和 ACK 怎么变化？

- **一句话**：SEQ=本报文第一个字节的流内偏移；ACK=期望对方下发的序号（之前全收到）；推进量=新收到的字节数（SYN/FIN 各占 1）。
- **抓包**：`packet/tcp-seq-ack.md` 的 10 行时间轴是标准答案素材。
- **展开**：累计确认的代价——中间丢一段，后面到了也只能重复确认；3 个重复 ACK → 快速重传；Wireshark 默认显示相对序号。

### 9. TCP 粘包/半包是怎么回事？怎么解决？

- **一句话**：TCP 是字节流，不保留 Write 的次数与大小，所以"一次 Write=一次 Read"不成立；解决靠应用层定义消息边界：定长 / 分隔符 / 长度前缀。
- **实验**：`tcp/stream`——背靠背两次 Write 5+5 字节，server 一次 Read 到 10 字节 `"helloworld"`；间隔 1 秒则是两次 Read 各 5 字节（真实输出在 `packet/tcp-byte-stream.md`）。
- **展开**：粘包这个词不准确（TCP 本来就没有"包"的概念），本质是**流式语义**；业界全是长度前缀（HTTP Content-Length、WebSocket 帧长、gRPC 5 字节前缀）；`io.ReadFull` 是对抗半包的正确姿势。

### 10. 滑动窗口 / 流量控制？

- **一句话**：接收方通过 TCP 头的 Window 字段实时通告"我还能收多少字节"，发送方未确认的在途数据不能超过它——防止快发送方淹没慢接收方。
- **展开**：流量控制是端到端的（保护接收方），拥塞控制是对网络的（保护链路）；实际发送量 = min(接收窗口, 拥塞窗口)；窗口为 0 时发送方停发并定期发探测包；`netstat`/`ss` 能看到 Recv-Q/Send-Q。

### 11. TCP 拥塞控制？

- **一句话**：发送方维护拥塞窗口 cwnd：慢启动（指数增）→ 拥塞避免（线性增）→ 丢包时乘性减（快速重传/恢复或超时回退到 1）。
- **展开**：目的不是可靠而是**不压垮网络**；在内核实现，应用（Go 代码）完全看不到；能说出 cwnd/rwnd/ssthresh、超时与三次重复 ACK 两条丢包路径的区别即可，校招不要求 BBR 细节。

### 12. TCP 重传？

- **一句话**：超时重传（RTO 到期重发最老未确认段，RTO 随 RTT 自适应）+ 快速重传（3 个重复 ACK 立即重发，不用等超时）+ SACK（选择性确认，告诉发送方哪些段已收到）。
- **展开**：回环抓包看不到重传（不丢包），这是"实验 + 面试"的天然分界：机制背下来，现象在真实弱网（或 `tc netem` 模拟丢包）才可见。

---

## 二、UDP

### 1. UDP 和 TCP 的区别？

- **一句话**：UDP 无连接、不可靠、不保序、无流控拥塞控制、头部 8 字节、保留数据报边界；TCP 全部相反、头部 20~60 字节、字节流无边界。
- **实验**：`udp/echo` vs `tcp/echo` 抓包对比——UDP 全程只有数据报（无 SYN/FIN/ACK）。
- **抓包**：`packet/udp.md`——两条 UDP 报文完成一次问答。
- **展开**：追问"UDP 快在哪"→ 不是代码快，是**省掉了握手（0-RTT 发数据）和确认重传的延迟与带宽**；追问场景 → DNS/音视频/游戏/QUIC。

### 2. 什么是数据报（Datagram）？

- **一句话**：UDP 的传输单位，一次 `Write` = 一条独立报文 = 对端一次 `Read` 收全；buf 比报文小则截断丢弃，不存在"下次再读"。
- **实验**：`udp/echo` 连发两个数据报，server 两次 ReadFromUDP 各收各的（对比 TCP 的 `helloworld` 粘包）。
- **展开**：边界保留是 UDP 与 TCP 最容易被忽略的区别；单个数据报上限 64KB，实际受 MTU/分片影响。

### 3. 为什么 DNS 用 UDP？

- **一句话**：一问一答小报文，UDP 免握手延迟低；可靠性靠应用层超时重传补；响应太大时设 TC 标志转 TCP。
- **实验**：`dns/rawquery` 手工发 UDP 查询。
- **展开**：能把"UDP+应用层重传"与 `udp/retry` 实验串起来讲，比背结论高一个层次；补充 DoT/DoH（DNS over TLS/HTTPS）说明现代演进。

### 4. UDP 可靠吗？要可靠怎么办？

- **一句话**：不可靠（可能丢、重、乱序）；要可靠要么用 TCP，要么在应用层自己做确认+重传+去重+排序，要么用 QUIC 这种"UDP 上自建可靠层"的方案。
- **实验**：`udp/retry`——server 随机丢 40%，client 超时重传；server 日志里 `DROP` 与 client 的 `timeout, retrying...` 一一对应；重传带来的**重复消息**问题正好引出"为什么 TCP 需要序号"。
- **展开**：说清"重传协议自己写有多麻烦"是加分项。

---

## 三、HTTP

### 1. HTTP 请求/响应的结构？

- **一句话**：请求行/状态行 + 头部（`Key: Value`，以空行结束）+ 可选 Body，行尾一律 `\r\n`。
- **实验**：`http/minparser`——客户端手工拼请求字节、服务端手工解析，60 行实现互操作（curl 也能访问）。
- **抓包**：`packet/http.md`——`curl -v` 与 tcpdump -A 的输出完全对应。
- **展开**：`Host` 头的作用（虚拟主机路由）；Content-Type 决定 Body 怎么解释。

### 2. Content-Length 为什么必须？

- **一句话**：TCP 没有消息边界，HTTP 靠 Content-Length（或 chunked/连接关闭）告诉读方"这条消息到哪结束"。
- **实验**：`http/minparser/server` 按 Content-Length 用 `io.ReadFull` 精确读 Body。
- **展开**：没有它且连接不关 → 客户端永远等；chunked 是"每块自带长度"的流式替代；HTTP/2 干脆用帧长。

### 3. HTTP Keep-Alive 是什么？

- **一句话**：一条 TCP 连接上串行跑多个请求/响应，省掉每次握手挥手；HTTP/1.1 默认开启。
- **实验**：`curl http://localhost:8081/hello http://localhost:8081/hello` 第二次显示 `Re-using existing connection`。
- **展开**：边界由 Content-Length 区分——所以它是 Keep-Alive 的前提；HTTP/1.1 的队头阻塞（前一个响应没完，后面的排队）由 HTTP/2 多路复用解决。

### 4. HTTP/1.1、HTTP/2、HTTP/3 的区别？

- **一句话**：1.1 文本+一条连接一个请求；2 二进制分帧+多路复用+HPACK，但仍跑在 TCP 上；3 换 QUIC(UDP)，彻底解决 TCP 队头阻塞，强制加密。
- **展开**：讲清两级队头阻塞——HTTP 层的（1.1 有，2 没有）和传输层的（2 有，3 没有）；版本协商靠 TLS ALPN。

### 5. HTTP 与 TCP 的关系？

- **一句话**：HTTP 是应用层消息格式，自身不传输；每个 HTTP 请求-响应都跑在一条 TCP 连接上（HTTPS 再加 TLS）。
- **实验**：`http/minparser` 用 `net.Listen` + 字节解析实现 HTTP，证明"HTTP 服务器=TCP 服务器+解析规则"。
- **抓包**：`packet/http.md` 的分层视图——IP 上面是 TCP，TCP 上面才是 HTTP。

---

## 四、TLS

### 1. TLS 是什么？解决什么问题？

- **一句话**：给 TCP 流加三个保证：机密性（对称加密）、完整性（AEAD 认证标签）、身份认证（证书链），位于应用层与 TCP 之间。
- **实验**：`tls/server+client`（`hello over TLS 1.3, cipher=TLS_AES_128_GCM_SHA256`）。
- **展开**：分层背板图：HTTP→TLS→TCP→IP；HTTPS = HTTP over TLS。

### 2. TLS 握手在做什么？

- **一句话**：协商版本与套件（ClientHello/ServerHello）→ 服务端出示证书、客户端验证 → ECDHE 双方各自算出同一把会话密钥（私钥与密钥从不上线）→ Finished 互证 → 之后全部是对称加密的 Application Data。
- **实验/抓包**：`packet/tls-handshake.md`——openssl `-state` 实测的 1.3/1.2 状态序列，Wireshark 里从第 6 个包起全是 Application Data。
- **展开**：为什么证书明文传输不泄密（证书本来就是公开的）；SNI 明文的原因（服务器要靠它选证书）。

### 3. 对称加密 vs 非对称加密，各用在哪？

- **一句话**：非对称（RSA/ECDHE）慢但不需要预共享秘密，用于握手期交换密钥与签名；对称（AES-GCM/ChaCha20）快，用于加密所有应用数据。
- **展开**：会话密钥"每次连接不同、由双方算出、不被传输"（ECDHE）；静态 RSA 密钥交换因无前向安全被 TLS 1.3 删除——"私钥泄漏也解不开过去的流量"就是前向安全。

### 4. TLS 1.2 和 1.3 的区别？

- **一句话**：1.3 握手 2-RTT→1-RTT（恢复 0-RTT）、密钥交换只留 (EC)DHE（强制前向安全）、只留 AEAD、握手从第 2 个报文起就加密、砍掉 CBC/RC4/压缩/重协商。
- **实测**：同一 server，Go 默认协商 `TLS 1.3 / TLS_AES_128_GCM_SHA256`，强制 1.2 时套件变成 `TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256`（名字长=协商项多）。
- **抓包**：1.2 的 Certificate 明文可见；1.3 的 Certificate 在加密区（`packet/tls-handshake.md`）。
- **展开**：0-RTT 的重放风险（早期数据只允许幂等请求）；"简化即安全"（少协商空间、少历史包袱）。

### 5. HTTPS 证书验证验什么？

- **一句话**：证书链能追到受信根 CA、域名匹配 SAN、在有效期内、未吊销。
- **实验**：`tls/gencert` 自签 + `tls/client` 用 RootCAs 手动信任；`openssl s_client` 报 `Verify return code: 18 (self-signed certificate)` 就是验证机制在工作。
- **展开**：`InsecureSkipVerify: true` 是跳过验证不是"解决验证"；中间人攻击的本质就是"没有可信链的证书被接受"。

---

## 五、SSE

### 1. SSE 是什么？

- **一句话**：建立在普通 HTTP 之上的 Server→Client 单向文本事件流——一个永不结束的 HTTP 响应体，用 `Content-Type: text/event-stream` 和空行分隔的事件格式约定。
- **实验**：`sse/server`（curl 实测输出：`200 OK` + `text/event-stream` + `chunked` + `id:/data:` 事件）。
- **展开**：必须能说出这句话：**SSE 不是 TCP、不是 WebSocket、不是新的传输层协议，它就是 HTTP**。

### 2. SSE 为什么能持续推送？

- **一句话**：HTTP 响应体没有长度上限，服务端不写完不结束，响应就一直"在路上"；每 flush 一次，客户端就收到一个 chunk。
- **抓包**：`packet/sse.md`——同一连接上每秒一个 PSH 段，方向恒为 server→client。

### 3. SSE 与 WebSocket 怎么选？

- **一句话**：只要服务端单向推 → SSE（纯 HTTP、浏览器自动重连、带 Last-Event-ID 续传）；要双向高频 → WebSocket（Upgrade 后独立帧协议、支持二进制）。
- **展开**：SSE 只能文本（UTF-8）；中间设备友好度（SSE 走普通 HTTP，WebSocket 需要代理支持 Upgrade）；对比表格见 `docs/sse.md`。

### 4. SSE 断线重连是怎么回事？

- **一句话**：浏览器 EventSource 内置自动重连——断开后按 `retry:` 指定的毫秒数重新 GET，并带 `Last-Event-ID` 头，服务端据此补发漏掉的事件。
- **展开**：重连就是全新的 TCP(→TLS)→HTTP 请求，没有任何特殊协议动作；`id:` 字段是服务端能"续传"的关键。

---

## 六、WebSocket

### 1. WebSocket 是什么？为什么需要 Upgrade？

- **一句话**：借 HTTP 握手（Upgrade 头 + 101 Switching Protocols）穿过 80/443 的防火墙与代理，之后在同一条 TCP 连接上切换成独立的双向帧协议。
- **实验**：`websocket/server+client`；curl 手工发 Upgrade 头得到真实 101（`Sec-WebSocket-Accept` 可手工验证）。
- **抓包**：`packet/websocket.md`——101 之前 Wireshark 显示 HTTP，之后显示 WebSocket 帧。

### 2. WebSocket 和 TCP 什么关系？帧里有什么？

- **一句话**：WebSocket 完全跑在一条 TCP 连接上，只是把字节流按帧（opcode + 长度 + 掩码 + payload）重新定义了消息边界。
- **展开**：opcode：0x1 Text / 0x2 Binary / 0x9 Ping / 0xA Pong / 0x8 Close / 0x0 分片；客户端帧必须掩码（防代理缓存投毒）；帧头自带长度 → 没有粘包问题。

### 3. Ping/Pong 干什么用的？全双工是什么意思？

- **一句话**：Ping/Pong 是控制帧，做心跳保活与存活探测（收到 Ping 必须回 Pong）；全双工指连接建立后双方都能随时主动发帧。
- **实验**：`websocket/client` 发 Ping 收 Pong、最后发 Close 帧（状态码 1000）再 TCP 挥手——三层关闭顺序清晰可见。
- **展开**：关闭分两层（Close 帧=应用层告别，FIN=传输层挥手）；gorilla 自动回 Pong、应用要自己做读超时与心跳周期。

---

## 七、DNS

### 1. DNS 是什么？解析流程？

- **一句话**：域名→IP 的应用层查询服务；你的 stub → 本地 DNS（有缓存直接回）→ 根 → TLD → 权威的递归/迭代查询链。
- **实验**：`dns/lookup`（系统解析器）、`dns/rawquery`（手工拼报文直发 1.1.1.1）。
- **抓包**：`packet/dns.md`——一来一回两条 UDP，Transaction ID 配对。

### 2. 为什么通常跑在 UDP 53 上？

- **一句话**：一问一答小报文不需要连接，UDP 省握手；可靠性靠应用层超时重传；响应超 512 字节（TC 标志）/区域传送/DoT/DoH 才用 TCP。
- **展开**：与 `udp/retry` 实验互证"应用层重传"模式；无连接所以要用 16 位 ID 匹配问答（也是 DNS 放大攻击的根源，可作加分谈资）。

### 3. A / AAAA / CNAME 记录？

- **一句话**：A=域名→IPv4；AAAA=域名→IPv6；CNAME=域名→另一个域名（别名链，最终以 A/AAAA 收尾）。
- **实测**：`go run ./dns/rawquery www.baidu.com 8.8.8.8` 输出 CNAME 链：`www.baidu.com →(type 5) www.a.shifen.com →(type 1) 103.235.46.x`。

### 4. DNS 缓存在哪？

- **一句话**：浏览器 → 操作系统 → 本地 DNS 服务器三级缓存，每条记录按 TTL 过期。
- **实验**：同一域名连续两次 `dns/rawquery`，第二次常抓不到新查询（命中缓存）；用随机子域名对比即可见新查询。
- **展开**：改解析记录后"全球生效等 TTL"；低 TTL=调度灵活但解析频繁。

---

## 八、ICMP

### 1. ping 用的是什么协议？

- **一句话**：ICMP Echo Request（Type 8）/ Echo Reply（Type 0），**不使用 TCP 也不使用 UDP**，直接跑在 IP 上（IP 头 Protocol=1），没有端口概念。
- **实验**：`icmp/ping`（实测回环 RTT 180µs、公网 103ms）。
- **抓包**：`packet/icmp.md`——只有一来一回两条报文，无握手、无端口字段。

### 2. ICMP 与 TCP/UDP 的关系？

- **一句话**：三者平级，都直接承载于 IP；区别是 TCP/UDP 面向进程（有端口、传数据），ICMP 是 IP 的控制面（传差错与控制消息，无端口）。
- **展开**：目标端口无人监听时对端回的 ICMP Port Unreachable 会让 UDP socket 收到 `connection refused`（`udp/retry` 里可复现）；traceroute 靠 TTL 超时的 ICMP 画路径。

### 3. 为什么 ping 要特殊权限？

- **一句话**：raw socket 发 ICMP 需要 root；macOS/BSD 给了非特权通道（`SOCK_DGRAM + IPPROTO_ICMP`），本仓库实验因此免 sudo。
- **展开**：能讲出"macOS 上 `x/net/icmp` 的 ListenPacket 会因 bind 报 EINVAL，要用 syscall 创建不绑定的 socket"这类实测细节，是把实验真正做过一遍的证据。

---

## 九、Protobuf 与 gRPC

### 1. Protobuf 是什么？为什么比 JSON 小/快？

- **一句话**：带 Schema 的二进制序列化格式；线上只有"字段编号 tag + 值"，没有字段名和结构符号，数字用 varint 变长编码。
- **实验**：`proto`——`id=1, name="kian"` 序列化为 8 字节 `08 01 12 04 6b 69 61 6e`，同数据 JSON 22 字节；逐字节解释（0x08=field1 varint，0x12=field2 长度前缀）在 `docs/proto.md`。
- **展开**：**field number 才是协议**——所以编号不可改不可复用；wire type 表；proto3 的默认值不序列化。

### 2. Proto 和 gRPC 是什么关系？

- **一句话**：Protobuf 只负责数据结构与序列化；gRPC 在其上定义 RPC 调用模型（方法路由、状态码、超时、四种流模式）并负责传输——两者不是一回事，可以分开用。
- **实验**：`grpc/user.proto` 里 message 与 service 的分工；生成物 `user.pb.go`（数据）vs `user_grpc.pb.go`（调用骨架）。

### 3. gRPC 跑在哪？一次调用发了什么？

- **一句话**：HTTP/2（通常 over TLS）；一次 Unary 调用 = 一个 POST `/包名.服务名/方法名`，Body 是"5 字节前缀 + protobuf"，结果通过 `grpc-status` trailer 返回。
- **实验**：`grpc/server+client`；`GODEBUG=http2debug=2` 的实测帧序列（SETTINGS → HEADERS → DATA → trailer `grpc-status: 0` → GOAWAY）在 `docs/grpc.md`。
- **展开**：为什么必须 HTTP/2——多路复用（一条连接并发多 RPC）、双向流、trailer 机制；5 字节前缀是 length-prefix 的又一例；HTTP/2 没有解决 TCP 层队头阻塞（引出 QUIC）。

### 4. .proto 从定义到运行的流水线？

- **一句话**：`.proto` → protoc（+语言插件）→ 生成 struct 与 client/server 接口 → 业务代码只填实现和调用，编解码与传输由运行时库完成。
- **实验**：本仓库生成命令在 README，生成代码已提交可对照阅读。
- **展开**：Schema 即契约：前后端/多语言靠同一份 .proto 对齐；版本演进规则（编号只加不改、预留 reserved）。

---

## 十、综合题（区分度高，建议全部过一遍）

1. **浏览器输入 https://example.com 到页面展示，发生了什么？**——按 `docs/architecture.md` 第三节的 6 步讲：DNS → TCP 握手 → TLS 1.3 握手 → HTTP 请求 → 响应渲染 → 连接复用/关闭；每步都能落到本仓库的某个实验与抓包。
2. **WebSocket/SSE/gRPC 都"长连接"，底层一样吗？**——SSE=普通 HTTP 响应不结束；WebSocket=Upgrade 后的独立帧协议；gRPC=HTTP/2 stream 复用。三者的"长连接"分别在 HTTP 层、切换协议后、HTTP/2 stream 层。
3. **一次 Write 之后对方立刻能 Read 到吗？**——不一定。数据先进内核缓冲，经 SEQ/ACK 才到对端；对端 Read 返回的时机取决于其调度与缓冲区（粘包实验）；Write 返回只表示"已拷贝进发送缓冲"。
4. **服务端大量 CLOSE_WAIT / 大量 TIME_WAIT 分别说明什么？**——CLOSE_WAIT=自己忘了 close（bug）；TIME_WAIT=自己主动关闭太多（短连接架构问题，考虑长连接或连接池）。
5. **HTTPS 能防什么、不能防什么？**——防窃听/篡改/冒充；不防"网站本身是坏的"（钓鱼站有合法证书）、不防终端木马、DNS 解析结果可能指向假站但证书验证会拦（除非用户点了忽略）；SNI/IP 仍暴露访问目标。
