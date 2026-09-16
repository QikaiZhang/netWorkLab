# TLS 概念文档

> 配套实验：`tls/gencert`、`tls/server`、`tls/client`，抓包现场见 [packet/tls-handshake.md](../packet/tls-handshake.md)。版本文史与 TLS 1.2 vs 1.3 见本文末尾章节。

## TLS 是什么、解决什么问题

TLS（Transport Layer Security）位于**传输层与应用层之间**（常被归为会话/表示层），给任何基于 TCP 的协议套上三个保证：

| 威胁 | 没有 TLS 时 | TLS 的对策 | 靠什么 |
|---|---|---|---|
| 窃听 | 中间人能读到明文（HTTP 密码直接裸奔） | **机密性**：对称加密 | 会话密钥 |
| 篡改 | 中间人可改包重发 | **完整性**：AEAD 认证加密 | GCM/ChaCha20-Poly1305 的认证标签 |
| 冒充 | 假网站、假 DNS、假网关 | **身份认证**：证书 | CA 签名的证书链 |

```text
HTTPS = HTTP
         ↓
        TLS      ← 加密/认证/完整性都在这层
         ↓
        TCP
         ↓
        IP
```

TLS 本身不传输应用数据，它只是把 TCP 的字节流"包装"成加密流。所以 HTTPS 服务器的应用层代码与 HTTP 几乎一样（对比 `http/server` 与 `tls/server`，只多了证书配置）。

## 两种加密算法各干什么

- **非对称加密（RSA / ECDHE）**：公钥加密、私钥解密。慢，但不需要提前共享秘密 → 用于**握手时安全地协商出会话密钥**、以及签名认证。
- **对称加密（AES-GCM / ChaCha20-Poly1305）**：双方用同一把密钥加解密。快几个数量级 → 用于**传输所有应用数据**。
- 为什么这么分：非对称太慢扛不住流量，对称密钥又没法明文发给对方。所以 TLS 的套路是"**用非对称把对称密钥安全送过去，之后全用对称**"。这个"会话密钥"每次连接都不同、不出现在线路上、由双方各自算出（ECDHE 双方交换的是公钥参数，密钥从未被传输）。

## 证书（Certificate）

证书 = **身份信息 + 公钥 + CA 的签名**。它回答"我确实是 localhost/example.com，这是我的公钥，有 CA 背书"。

- 浏览器/客户端内置了一组受信任 CA 的根证书。收到服务器证书后：验签名（逐级验到根）、验有效期、验域名匹配（SAN 字段）、查吊销状态。
- `tls/gencert` 生成的是**自签发**证书：`IsCA: true`，自己给自己签名——所以 `openssl s_client` 报 `Verify return code: 18 (self-signed certificate)`；`tls/client` 的做法是把这张证书手动加入信任池（`RootCAs`），这才是测试环境绕开 CA 的正确姿势（`InsecureSkipVerify: true` 是关掉校验，永远不要带到生产）。
- 抓包里证书是**明文**传输的（Certificate 消息不加密）——证书本来就是要给所有人看的公开信息。加密保护的是之后的通信内容。

## TLS 握手在做什么

详见 [packet/tls-handshake.md](../packet/tls-handshake.md)。本质是四件事：

```text
1. 协商：   客户端说"我会这些版本/套件" → 服务端挑一个（ClientHello / ServerHello）
2. 认证：   服务端出示证书，客户端验证身份（服务端也可要求客户端证书，双向认证）
3. 密钥交换：双方用 ECDHE 各自算出同一把会话密钥（线路上只见公钥参数）
4. 确认：   Finished 消息用会话密钥加密，证明密钥一致、握手未被篡改
```

之后所有数据都作为 **Application Data 记录**加密传输。抓包里从某个报文开始全是 `Application Data`，看不到 HTTP 头——这就是"TLS 在 HTTP 与 TCP 之间"的直观证据。

## Go 代码与协议的关系

- `srv.ListenAndServeTLS(cert, key)` = TCP Listen + Accept，每个连接先跑 TLS 握手，之后把解密后的 HTTP 字节流交给 http 包。加密完全在 `crypto/tls` 内部。
- `tls.Dial` / `http.Transport{TLSClientConfig}` = 连接 + 握手 + 验证证书。`r.TLS.Version`、`CipherSuite` 就是协商结果（实验输出 `TLS 1.3, TLS_AES_128_GCM_SHA256`）。
- 证书、密钥交换、加解密、重协商、会话恢复全部由标准库完成——**应用代码永远不碰密钥学参数**，这正是"标准库替你做了什么"的又一例。

## 版本文史：SSL → TLS

| 版本 | 年代 | 状态 |
|---|---|---|
| SSL 2.0 | 1995 | 已废弃，严重缺陷 |
| SSL 3.0 | 1996 | 已废弃（POODLE 攻击） |
| TLS 1.0 | 1999 | 已废弃（BEAST 等攻击，PCI-DSS 禁用） |
| TLS 1.1 | 2006 | 已废弃（2021 年 RFC 8996 正式移除） |
| TLS 1.2 | 2008 | **当前主流底线**，仍广泛使用 |
| TLS 1.3 | 2018 | 当前最新，只保留安全套件 |

注意：SSL 是旧名，TLS 是它的标准化后继；今天说"SSL 证书"指的都是 TLS 证书。

## TLS 1.2 vs TLS 1.3（不是"1.3 更安全"一句话能带过的）

### 握手往返：2-RTT → 1-RTT（恢复时 0-RTT）

```text
TLS 1.2                        TLS 1.3
C → ClientHello                C → ClientHello + key_share ─┐ (把密钥交换并进第一个包)
C ← ServerHello                C ← ServerHello + key_share   │
C ← Certificate, ServerKeyExchange,  ← {EncryptedExtensions, Certificate, CertificateVerify, Finished}  ← 从这里开始已加密
      ServerHelloDone                 ↓
C → ClientKeyExchange, ChangeCipherSpec, Finished   ← 第三条消息
C ← ChangeCipherSpec, Finished
（第 3 个 RTT 后才能发数据）      （第 2 个报文之后客户端即可发数据）
```

- TLS 1.2 要**两轮往返**才能发应用数据；TLS 1.3 把密钥交换参数（key_share）直接塞进 ClientHello，**一轮往返**完成握手。
- 会话恢复时 TLS 1.3 支持 **0-RTT**：客户端用缓存的 psk 在第一个报文里就带上应用数据。代价是 0-RTT 数据没有防重放保证（早期数据可能被重放），所以只允许幂等请求。

### 套件与算法：从"菜单很长"到"只留硬菜"

- TLS 1.2 的套件名混合了四种角色：`TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256` = 密钥交换(ECDHE) + 证书签名(ECDSA) + 批量加密(AES-GCM) + 摘要(SHA256)。
- TLS 1.3 砍掉了协商内容：密钥交换**只允许 (EC)DHE**（支持前向安全），批量加密**只允许 AEAD**，摘要并入 HKDF。套件名缩短为 `TLS_AES_128_GCM_SHA256`——因为其中三样已经是定死的选择。
- 被删除的：RSA 密钥交换（只有静态 RSA，无前向安全）、CBC 模式（漏洞频出）、RC4、DES、MD5、压缩、重协商。

### 为什么 1.3 能这么简化

近十年密码学结论已经收敛：前向安全的 ECDHE + AEAD 是唯一值得保留的组合。握手里不再有"协商出弱算法"的空间，历史包袱（ ChangeCipherSpec、Session renegotiation）直接移除，握手消息也从第二个报文开始就加密——中间人能看到的明文更少，握手更快也更安全。**简化本身就是安全性的来源。**

### 抓包对照

TLS 1.3 握手里，`Certificate` 等消息已经加密（Wireshark 显示为 `Encrypted Extensions/Certificate...` 或 Application Data），明文可见的只有 ClientHello/ServerHello 的 SNI 与套件列表；TLS 1.2 握手则能明文看到证书。本仓库实验 server 默认协商 1.3（Go 客户端首选 1.3），强制 1.2 时实测套件为 `TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256`。

## 会话恢复（简述）

- TLS 1.2：session ID / session ticket，恢复仍需 1-RTT。
- TLS 1.3：PSK（pre-shared key）+ 0-RTT early data。CDN 场景下"第二次访问秒回"就靠它。
