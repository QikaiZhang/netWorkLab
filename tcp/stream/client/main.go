// tcp/stream/client: 用两种写入节奏向 server 发送同样的两个词，观察 server 端 Read 的差异。
package main

import (
	"fmt"
	"log"
	"net"
	"time"
)

func main() {
	// 模式一：两次 Write 背靠背发出，中间不等待。
	c1, err := net.Dial("tcp", "localhost:9001")
	if err != nil {
		log.Fatal(err)
	}
	c1.Write([]byte("hello"))
	c1.Write([]byte("world"))
	time.Sleep(200 * time.Millisecond)
	c1.Close()

	// 模式二：两次 Write 之间隔 1 秒。
	c2, err := net.Dial("tcp", "localhost:9001")
	if err != nil {
		log.Fatal(err)
	}
	c2.Write([]byte("hello"))
	time.Sleep(1 * time.Second)
	c2.Write([]byte("world"))
	time.Sleep(200 * time.Millisecond)
	c2.Close()
	fmt.Println("sent hello/world in two modes; check server output")
}
