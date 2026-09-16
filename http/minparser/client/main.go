// http/minparser/client: 用裸 TCP 连接发送手工拼装的 HTTP 请求字节，打印原始响应字节。
package main

import (
	"fmt"
	"io"
	"log"
	"net"
)

func main() {
	conn, err := net.Dial("tcp", "localhost:8082")
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	// 手工拼出完整的 HTTP 请求字节流
	req := "POST /echo HTTP/1.1\r\n" +
		"Host: localhost:8082\r\n" +
		"Content-Type: text/plain\r\n" +
		"Content-Length: 5\r\n" +
		"\r\n" +
		"hello"
	fmt.Printf("--- request bytes ---\n%s\n", req)

	if _, err := conn.Write([]byte(req)); err != nil {
		log.Fatal(err)
	}

	resp, err := io.ReadAll(conn)
	if err != nil && err != io.EOF {
		log.Fatal(err)
	}
	fmt.Printf("--- response bytes (%d) ---\n%s\n", len(resp), resp)
}
