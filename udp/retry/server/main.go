// udp/retry/server: 监听 :9003，随机丢弃约 40% 的数据报，模拟丢包网络。
package main

import (
	"fmt"
	"log"
	"math/rand"
	"net"
)

func main() {
	conn, err := net.ListenUDP("udp", &net.UDPAddr{Port: 9003})
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()
	fmt.Println("lossy udp server listening on :9003 (drops ~40%)")

	buf := make([]byte, 1024)
	for {
		n, remote, err := conn.ReadFromUDP(buf)
		if err != nil {
			log.Fatal(err)
		}
		msg := string(buf[:n])
		if rand.Float64() < 0.4 {
			fmt.Printf("DROP  %q from %s\n", msg, remote)
			continue
		}
		fmt.Printf("RECV  %q from %s -> ack\n", msg, remote)
		if _, err := conn.WriteToUDP([]byte("ack "+msg), remote); err != nil {
			log.Println(err)
		}
	}
}
