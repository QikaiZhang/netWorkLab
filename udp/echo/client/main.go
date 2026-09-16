// udp/echo/client: 向 server 发送两个数据报并等待回显，观察数据报边界。
package main

import (
	"fmt"
	"log"
	"net"
	"time"
)

func main() {
	server, err := net.ResolveUDPAddr("udp", "localhost:9002")
	if err != nil {
		log.Fatal(err)
	}
	conn, err := net.DialUDP("udp", nil, server)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()
	fmt.Printf("local addr: %s\n", conn.LocalAddr())

	for i := 1; i <= 2; i++ {
		msg := fmt.Sprintf("datagram-%d", i)
		if _, err := conn.Write([]byte(msg)); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("sent: %q\n", msg)

		buf := make([]byte, 1024)
		conn.SetReadDeadline(time.Now().Add(3 * time.Second))
		n, err := conn.Read(buf)
		if err != nil {
			log.Fatalf("no echo received: %v", err)
		}
		fmt.Printf("echo %d bytes: %q\n", n, buf[:n])
	}
}
