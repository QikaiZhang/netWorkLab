# 实验：观察 UDP 数据报

对应实验：`udp/echo`。对比 TCP 抓包，注意三件事：没有握手、每个数据报独立成包、报文边界保留。

## 运行与抓包

```bash
# 终端 1：抓包（先启动）
sudo tcpdump -i lo0 -nn port 9002
# 或 Wireshark 选 lo0，display filter: udp.port == 9002

# 终端 2：go run ./udp/echo/server
# 终端 3：go run ./udp/echo/client
```

## 你应该看到什么

回显两个数据报，完整抓包只有 **4 条**（对比 TCP echo 同样场景至少 10 条以上）：

```text
12:00:00.000001 IP 127.0.0.1.62907 > 127.0.0.1.9002: UDP, length 10
12:00:00.000003 IP 127.0.0.1.9002 > 127.0.0.1.62907: UDP, length 10
12:00:00.000100 IP 127.0.0.1.62907 > 127.0.0.1.9002: UDP, length 10
12:00:00.000102 IP 127.0.0.1.9002 > 127.0.0.1.62907: UDP, length 10
```

没有 `Flags [S]`、没有 `Flags [F]`、没有 ACK——**无连接**：不发握手包，不发关闭包，Client 进程退出连接就"没了"，server 端什么状态变化都没有（netstat 里查不到任何 UDP 连接状态）。

Wireshark 中点开一条 UDP 报文，字段自上而下：

```text
User Datagram Protocol
    Source Port:        62907
    Destination Port:   9002
    Length:             18      ← 头部 8 字节 + 数据 10 字节
    Checksum:           0x....  ← 回环上常为 0 或未计算（offload）
    [data: "datagram-1"]
```

展开后下一层就是 IP 头（`Internet Protocol Version 4`），再下一层是以太帧——这就是"UDP 只比 IP 多一个端口"的直观形态。

## 数据报边界保留

`udp/echo` 的实测输出：

```text
# client
sent: "datagram-1"
echo 10 bytes: "datagram-1"
sent: "datagram-2"
echo 10 bytes: "datagram-2"

# server
datagram 10 bytes from 127.0.0.1:62907: "datagram-1"
datagram 10 bytes from 127.0.0.1:62907: "datagram-2"
```

把 client 改成背靠背连发两次（去掉中间的等待和读回显）再跑 server，会看到 server 仍然是**两次 ReadFromUDP 各 10 字节**——与 TCP 字节流实验（`helloworld` 粘成一次 Read 10 字节）形成直接对照：

- TCP：`Write` 次数不保留，读到的可能合并（粘包）。
- UDP：`Write` 次数=报文个数=`Read` 次数，每条独立；但如果你的 buf 比数据报小，**超出部分直接截断丢弃**（不是留到下次读）。

## 丢包去哪了

回环上几乎不丢包。要模拟真实网络，直接跑 `udp/retry` 实验（server 会随机丢弃收到的数据报）：你会看到 client 等待超时、重发，而线路上**被丢的报文没有任何痕迹**——没有 RST、没有 ICMP，接收方应用程序和内核都根本不知道曾有过这个包。这就是"UDP 尽力而为"的含义。

（补充：只有路由器/主机明确报错时才会有 ICMP 提示，例如目标端口无人监听会回 ICMP Port Unreachable——Go 中表现为 `ReadFromUDP` 返回 `connection refused` 错误。）
