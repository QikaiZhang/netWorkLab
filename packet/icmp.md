# 实验：抓包看 ICMP Echo Request / Reply

对应实验：`icmp/ping`。观察目标：ping 不经过任何端口、不经过 TCP/UDP，直接是 IP → ICMP。

## 运行与抓包

```bash
# 终端 1：抓包（对公网抓物理网卡，对回环抓 lo0）
sudo tcpdump -i en0 -nn icmp
# 或 Wireshark 选 Wi-Fi，display filter: icmp

# 终端 2：
go run ./icmp/ping 1.1.1.1
```

## 真实运行结果（实测）

```text
sent Echo Request to 1.1.1.1 (18 bytes)
got Echo Reply from 1.1.1.1: id=1 seq=1 data="HELLO-ICMP" rtt=103.262833ms
```

回环版本（`go run ./icmp/ping 127.0.0.1`）：

```text
sent Echo Request to 127.0.0.1 (18 bytes)
got Echo Reply from 127.0.0.1: id=1 seq=1 data="HELLO-ICMP" rtt=180.833µs
```

## tcpdump 输出（你应该看到什么）

```text
12:00:00.000001 IP 192.168.125.93 > 1.1.1.1: ICMP echo request, id 1, seq 1, length 18
12:00:00.103000 IP 1.1.1.1 > 192.168.125.93: ICMP echo reply, id 1, seq 1, length 18
```

Wireshark 展开的层次结构（与 TCP/UDP 实验对比的关键就在这）：

```text
Internet Protocol Version 4
    Protocol: ICMP (1)          ← 不是 6(TCP) 也不是 17(UDP)
    Source: 192.168.125.93
    Destination: 1.1.1.1
Internet Control Message Protocol
    Type: 8 (Echo (ping) request)
    Code: 0
    Checksum: 0x....            ← icmp.Marshal 自动计算
    Identifier: 1               ← 匹配问答用（类似端口的"影子"，但不是端口）
    Sequence: 1
    Data (10 bytes): "HELLO-ICMP"
```

三条最容易在面试里用上的观察：

1. **没有端口**：报文里找不到任何 16 位端口号字段。回环上同时 ping 一百次也不冲突，因为 Identifier/Sequence 承担了匹配职责。
2. **一来一回就是全部**：没有握手（对比 TCP 至少 3 条）、没有关闭序列（对比 FIN×2 + ACK×2）。RTT 就是两条报文的时间差——`rtt=103ms` 全部来自线路。
3. **请求与响应的 Data 完全相同**：`"HELLO-ICMP"` 原样返回。协议规定 Echo 必须回显数据，这也是有些网络禁 ping（丢弃 Echo Request）却仍能上网的原因——ICMP 只控制，不承载数据。

## 无权限时的表现

如果平台拒绝非特权 ICMP，`syscall.Socket` 会直接报错，例如 Linux 上无 root 时：

```text
socket: operation not permitted
```

解决：`sudo go run ./icmp/ping 1.1.1.1`（改用 raw socket），详见 [docs/icmp.md](../docs/icmp.md) 的权限说明。macOS 上本实验无需 sudo。
