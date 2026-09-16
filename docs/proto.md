# Protobuf 概念文档

> 配套实验：`proto/`。Protobuf 与网络传输无关——它是**序列化格式**，通常作为 gRPC 的消息编码出现，也可以自己跑在任何字节通道上（TCP/TCP 文件/HTTP body 都行）。

## Protobuf 是什么

三个词概括：**Schema、二进制、代码生成**。

1. 先用 `.proto` 文件声明数据结构（消息有哪些字段、什么类型、编号是几）——这就是 Schema。
2. 序列化成**紧凑的二进制**：没有字段名、没有分隔符，只有字段编号 + 值。
3. `protoc` 编译器读取 Schema，**生成**各语言的 Struct/类与序列化代码，两端 import 同一份生成代码，天然对齐。

## 最小实验

`proto/user.proto`：

```proto
syntax = "proto3";
package demo;
option go_package = "network-lab/proto/userpb;userpb";

message User {
  int64 id = 1;      // 字段编号（field number）才是线上协议的一部分！
  string name = 2;
}
```

生成命令（生成代码已提交到 `proto/userpb/user.pb.go`）：

```bash
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
protoc --go_out=. --go_opt=module=network-lab proto/user.proto
```

`go run ./proto` 的实测输出：

```text
proto  (8 bytes): 080112046b69616e
unmarshal: id=1 name=kian
json   (22 bytes): {"id":1,"name":"kian"}
```

## 二进制里到底是什么（8 字节逐个解释）

`id=1, name="kian"` 序列化为 `08 01 12 04 6b 69 61 6e`：

```text
08     tag: (field_number=1 << 3) | wire_type=0(varint)   → "接下来是 1 号字段的 varint"
01     value: id = 1
12     tag: (field_number=2 << 3) | wire_type=2(长度前缀)  → "接下来是 2 号字段的字符串"
04     length: 4 字节
6b 69 61 6e    "kian" 的 ASCII
```

对比 JSON 的 22 字节（`{"id":1,"name":"kian"}`），Protobuf 省掉了什么：

- **字段名**（`"id":` `"name":`）→ 用 1 个字节的编号 tag 代替
- **结构符号**（`{ } " "`）→ 用 wire type 隐式声明
- 数字用 **varint** 变长编码，小数字只占 1 字节（255 以内的 int 只需 1 字节，而不是定长 8 字节）

解析端拿到 `08` 就知道"1 号字段、varint 类型"，不需要任何外部信息——**field number 才是协议**。所以 `.proto` 演进的铁律是：**已用的字段编号永远不能改、不能复用**（改了类型或编号，新旧版本解析出来的就是另一个字段的数据）。

## wire type 速查

| wire type | 含义 | 用于 |
|---|---|---|
| 0 | varint 变长整数 | int32/64, bool, enum |
| 1 | 64-bit 定长 | double, fixed64 |
| 2 | length-delimited | string, bytes, 嵌套 message |
| 5 | 32-bit 定长 | float, fixed32 |

嵌套 message 的编码就是递归：子 message 按 wire type 2 写成"长度 + 子字节串"。

## Proto vs JSON

| | Protobuf | JSON |
|---|---|---|
| 体积 | 小（无字段名、varint） | 大（每个字段名都重复出现） |
| 解析速度 | 快（定式二进制解析） | 慢（文本扫描+字符串比较） |
| Schema | 强制（.proto 是唯一事实源） | 可选（JSON Schema 少有人用） |
| 人类可读 | 否（需 prototext/解码器） | 是 |
| 浏览器/调试友好 | 弱 | 强 |
| 适用 | 内部服务间高性能 RPC | 对外 API、配置、调试 |

## 生成代码在做什么

打开 `proto/userpb/user.pb.go` 可以看到：`User` 只是一个带 `protobuf:"..."` 标签的 struct，真正的编解码在 `google.golang.org/protobuf/proto` 里——生成代码是"结构描述 + 快速路径"，运行时库负责反射式的通用编解码。理解到"**生成的只是 Go struct + 描述符**"这个层面就够面试用了。

## 与 gRPC 的关系（预告）

> **Proto 和 gRPC 不是同一个东西。**

- Protobuf 只管：数据结构定义 + 二进制序列化。
- gRPC 在此之上：定义 service（RPC 方法），负责把"调用"编码成消息、跑在 HTTP/2 上传输。

Phase 10 会把 `User` 变成一个真正的远程调用。
