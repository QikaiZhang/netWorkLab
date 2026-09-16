// udp/retry/client: 发送带编号的消息，超时 800ms 未收到 ack 就重传，最多 4 次。
package main

import (
	"fmt"
	"log"
	"net"
	"time"
)

func main() {
	server, err := net.ResolveUDPAddr("udp", "localhost:9003")
	if err != nil {
		log.Fatal(err)
	}
	conn, err := net.DialUDP("udp", nil, server)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	buf := make([]byte, 1024)
	for i := 1; i <= 5; i++ {
		msg := fmt.Sprintf("msg-%d", i)
		sendWithRetry(conn, []byte(msg), buf)
	}
}

func sendWithRetry(conn *net.UDPConn, msg []byte, buf []byte) {
	for attempt := 1; attempt <= 4; attempt++ {
		fmt.Printf("send %q (attempt %d)\n", msg, attempt)
		if _, err := conn.Write(msg); err != nil {
			log.Fatal(err)
		}
		conn.SetReadDeadline(time.Now().Add(800 * time.Millisecond))
		n, err := conn.Read(buf)
		if err == nil {
			fmt.Printf("ok: %q\n", buf[:n])
			return
		}
		if ne, ok := err.(net.Error); ok && ne.Timeout() {
			fmt.Println("  timeout, retrying...")
			continue
		}
		log.Fatal(err)
	}
	fmt.Printf("give up: %q\n", msg)
}
