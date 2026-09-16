// udp/echo/server: 监听 :9002，把收到的数据报原样发回。
package main

import (
	"fmt"
	"log"
	"net"
)

func main() {
	addr, err := net.ResolveUDPAddr("udp", ":9002")
	if err != nil {
		log.Fatal(err)
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()
	fmt.Println("udp echo server listening on :9002")

	buf := make([]byte, 1024)
	for {
		n, remote, err := conn.ReadFromUDP(buf)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("datagram %d bytes from %s: %q\n", n, remote, buf[:n])
		if _, err := conn.WriteToUDP(buf[:n], remote); err != nil {
			log.Println(err)
		}
	}
}
