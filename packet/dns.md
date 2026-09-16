# 实验：抓一个真实的 DNS 查询

对应实验：`dns/lookup`、`dns/rawquery`。观察目标：DNS 报文的 Header/Question/Answer 三段结构，以及"查询走 UDP"。

## 运行与抓包

```bash
# 终端 1：抓包（对公网 DNS 抓真实接口；对本地 DNS 抓回环）
sudo tcpdump -i en0 -nn port 53
# 或 Wireshark 选 Wi-Fi/以太网接口，display filter: dns

# 终端 2：
go run ./dns/rawquery example.com 1.1.1.1
```

## 真实运行结果（实测）

```text
query 29 bytes -> 1.1.1.1:53
response 61 bytes: id=57589 rcode=0 questions=1 answers=2
  answer: type=1 ttl=199 data=104.20.23.154
  answer: type=1 ttl=199 data=172.66.147.243
```

查询 `www.baidu.com` 时能看到 CNAME 链：

```text
query 31 bytes -> 8.8.8.8:53
response 116 bytes: id=33393 rcode=0 questions=1 answers=4
  answer: type=5 ttl=155 data=www.baidu.com
  answer: type=5 ttl=19  data=www.a.shifen.com
  answer: type=1 ttl=13  data=103.235.46.115
  answer: type=1 ttl=13  data=103.235.46.102
```

## Wireshark 里的报文

抓到的两条 UDP 报文（没有握手、没有挥手，一来一回就结束）：

```text
No.1  192.168.x.x.5xxxx → 1.1.1.1:53  Len=29
      Domain Name System (query)
        Transaction ID: 0xe0f5                 ← buildQuery 里的随机 id
        Flags: 0x0100 Standard query           ← RD=1：请替我递归
        Questions: 1
        Queries
          example.com: type A, class IN
            Name: example.com
            [Name Length: 11] [Label Count: 2]  ← 3example3com0 的编码
            Type: A (1)
            Class: IN (0x0001)

No.2  1.1.1.1:53 → 192.168.x.x.5xxxx  Len=61
      Domain Name System (response)
        Transaction ID: 0xe0f5                 ← 和查询相同！客户端靠它匹配
        Flags: 0x8180 Standard query response  ← QR=1(响应), RA=1(支持递归), RCODE=0(成功)
        Answers: 2
        Answers
          example.com: type A, class IN, addr 104.20.23.154
            Name: example.com  (指向偏移 0x000c 的压缩指针)
            Type: A (1)
            TTL: 199                            ← 缓存 199 秒
            Address: 104.20.23.154
```

对照观察点：

1. **传输层只有两条 UDP**：没有 SYN/SYN-ACK/ACK（对比 TCP），也没有 FIN——"无连接、一来一回"这就是 DNS 走 UDP 的直接证据。
2. **Transaction ID 匹配**：UDP 无连接，客户端可能同时发多个查询，全靠这个 16 位 ID 配对——应用层自己实现"请求-响应关联"的例子。
3. **名字压缩**：Answer 里的 Name 显示为指针（`0xC00C` → Question 区），Wireshark 会标注 "pointer"。
4. **CNAME 链**：查 www.baidu.com 时 Answers 先给别名再给 A 记录——一次查询可能带出多级答案。

## lookup 实验：操作系统缓存层

`go run ./dns/lookup example.com` 的实测输出（记录会随 CDN 调度变化）：

```text
LookupHost(example.com):
  2606:4700:10::6814:179a AAAA (IPv6)
  2606:4700:10::ac42:93f3 AAAA (IPv6)
  172.66.147.243  A (IPv4)
  104.20.23.154   A (IPv4)
CNAME: example.com.
```

连续执行两次，第二次在 Wireshark 里**经常看不到新的 DNS 报文**——答案命中了操作系统或本地 DNS 的缓存。用不常见的域名（如随机子域名）对比，每次都能抓到新查询，这是演示"DNS 缓存"最直观的方法。
