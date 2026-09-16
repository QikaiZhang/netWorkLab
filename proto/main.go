// proto/main.go: 演示 Protobuf 的 Marshal/Unmarshal 与二进制编码，并与 JSON 对比。
package main

import (
	"encoding/json"
	"fmt"
	"log"

	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"

	pb "network-lab/proto/userpb"
)

func main() {
	u := &pb.User{Id: 1, Name: "kian"}

	// Protobuf: struct -> binary
	bin, err := proto.Marshal(u)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("proto  (%d bytes): %x\n", len(bin), bin)

	// 逐字节解码：每个字段 = (field_number << 3) | wire_type 的 tag + 数据
	// 0x08 = field 1, varint     -> 0x01
	// 0x12 = field 2, length-delimited -> 0x04 "kian"
	for _, b := range bin {
		fmt.Printf("  %02x", b)
	}
	fmt.Println()

	// binary -> struct
	back := &pb.User{}
	if err := proto.Unmarshal(bin, back); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("unmarshal: id=%d name=%s\n", back.GetId(), back.GetName())

	// 同数据的 JSON 对比
	js, _ := json.Marshal(u)
	fmt.Printf("json   (%d bytes): %s\n", len(js), js)

	// 文本形式（调试用）
	txt, _ := prototext.Marshal(u)
	fmt.Printf("text: %s\n", txt)
}
