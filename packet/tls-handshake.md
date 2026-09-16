# 实验：抓 TLS 握手，看清"加密从哪一刻开始"

对应实验：`tls/server` + `tls/client`。观察目标：握手哪些消息是明文、从哪条报文开始变成 `Application Data`。

## 运行与抓包

```bash
# 前置：go run ./tls/gencert 生成自签证书

# 终端 1：抓包
sudo tcpdump -i lo0 -nn port 8443
# 或 Wireshark 选 lo0，display filter: tls
# 只看握手消息类型: tls.handshake.type == 1（ClientHello）、== 2（ServerHello）
# 只看证书:         tls.handshake.type == 11
# 只看应用数据:     tls.record.content_type == 23

# 终端 2：go run ./tls/server
# 终端 3：go run ./tls/client   （或 curl -sk https://localhost:8443/）
```

## 不用抓包工具也能看到的握手序列（实测）

`openssl s_client` 的 `-state` 参数会打印客户端侧的握手状态机流转，本仓库实测：

TLS 1.3（默认协商结果）：

```text
SSL_connect:before SSL initialization
SSL_connect:SSLv3/TLS write client hello
SSL_connect:SSLv3/TLS read server hello
SSL_connect:TLSv1.3 read encrypted extensions
SSL_connect:SSLv3/TLS read server certificate
SSL_connect:TLSv1.3 read server certificate verify
SSL_connect:SSLv3/TLS read finished
SSL_connect:SSLv3/TLS write change cipher spec
SSL_connect:SSLv3/TLS write finished
```

TLS 1.2（强制 `-tls1_2`）：

```text
SSL_connect:before SSL initialization
SSL_connect:SSLv3/TLS write client hello
SSL_connect:SSLv3/TLS read server hello
SSL_connect:SSLv3/TLS read server certificate
SSL_connect:SSLv3/TLS read server key exchange
SSL_connect:SSLv3/TLS read server done
SSL_connect:SSLv3/TLS write client key exchange
SSL_connect:SSLv3/TLS write change cipher spec
SSL_connect:SSLv3/TLS write finished
SSL_connect:SSLv3/TLS read change cipher spec
SSL_connect:SSLv3/TLS read finished
```

对照读法（与 [docs/tls.md](../docs/tls.md#tls-12-vs-tls-13不是-13-更安全-一句话能带过的) 一一对应）：

- 1.3 在 **read server hello** 之后立刻进入加密区（encrypted extensions / certificate / finished 全是密文，客户端用自己的 key_share 解密验证）；1.2 的 certificate、key exchange 全程明文。
- 1.3 客户端**第一次发送就能带上密钥交换参数**（key_share 塞在 ClientHello 里），1-RTT 完成；1.2 客户端要等 server hello done 之后才能发 client key exchange，2-RTT。
- `change cipher spec` 在 1.3 里只是兼容性占位（真协商 1.3 时它不携带语义），1.2 里它才是"切换加密"的信号。

## Wireshark 里你应该看到什么

一次完整的 HTTPS 请求（TLS 1.3）在过滤器 `tls` 下的时间轴：

```text
No.  Info
1    50123 → 8443  SYN          （TCP 三次握手照旧，TLS 之下永远是 TCP）
2    8443 → 50123  SYN, ACK
3    50123 → 8443  ACK
4    C → S  Client Hello            ← 明文：TLS 版本列表、cipher suites、SNI=localhost、key_share
5    S → C  Server Hello            ← 明文：选定 TLS 1.3 与套件、服务端 key_share
6    S → C  Application Data        ← 已加密！内含 EncryptedExtensions/Certificate/CertificateVerify/Finished
7    C → S  Application Data        ← 客户端 Finished
8    C → S  Application Data        ← HTTP 请求 "GET / HTTP/1.1..."（密文）
9    S → C  Application Data        ← HTTP 响应（密文）
...
```

TLS 1.2 的差别：第 6 条位置上是明文的 `Certificate`（能在 Wireshark 里直接展开证书字段），以及单独的 `Client Key Exchange / Change Cipher Spec / Finished` 明文握手记录。

## 逐字段看 ClientHello（Wireshark 展开）

```text
Handshake Type: Client Hello (1)
    Version: TLS 1.2 (0x0303)          ← 字段名兼容旧版；真实版本看 supported_versions 扩展
    Extension: server_name (SNI)       ← 要访问的域名（明文！这就是 ECH 想加密的东西）
    Extension: supported_versions      ← TLS 1.3, TLS 1.2（客户端能力）
    Extension: key_share               ← ECDHE 公钥参数（X25519 等）
    Cipher Suites (x 个)               ← 客户端支持的套件列表
```

- **SNI 是明文**：一台服务器上托管多个 HTTPS 站点时，服务器要在看到 SNI 后才能选出对应的证书——所以它无法在握手时加密。
- `key_share` 是客户端的 ECDHE 公钥；ServerHello 返回服务端的 ECDHE 公钥。**双方各自的私钥从未上线**，这就是会话密钥"被算出来而不是传过去"的含义。

## 证书在哪、如何验证

`go run ./tls/client` 的实测输出：

```text
negotiated: TLS 1.3, cipher=TLS_AES_128_GCM_SHA256
server cert CN=localhost DNSNames=[localhost]
body: hello over TLS 1.3, cipher=TLS_AES_128_GCM_SHA256
```

客户端校验三件事：证书链能追溯到信任根（实验中是手动把自签证书加入 RootCAs）、域名匹配 SAN（`localhost`）、在有效期内。生产环境由系统根证书库完成这一切。用 `openssl s_client` 直接连会得到 `Verify return code: 18 (self-signed certificate)`——它不信任自签 CA，这正是验证机制在起作用。

## 常见问题

- **为什么抓不到 HTTP 头**：握手完成后所有字节都是 TLS 记录（content_type=23 Application Data），Wireshark 无法再解析出 HTTP 层。想看 HTTP 明文就用 8081 的明文实验。
- **能看到密钥吗**：抓包只有公钥参数。调试时可给程序设 `SSLKEYLOGFILE`（Go 1.20+ 的 `tls.Config.KeyLogWriter`）导出会话密钥，再在 Wireshark → Preferences → Protocols → TLS 里配置，即可解密自己抓的包——仅限调试环境。
- **回环抓包为什么这么快**：本机没有真实 RTT，1-RTT/2-RTT 的差距要在真实网络（如 `curl -w '%{time_appconnect}' https://example.com` 对比 1.2/1.3 站点）上才明显。
