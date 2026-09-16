// tcp/echo/server: 监听 :9000，把收到的字节原样写回客户端。
package main

import (
	"fmt"
	"log"
	"net"
)

func main() {
	ln, err := net.Listen("tcp", ":9000")
	if err != nil {
		log.Fatal(err)
	}
	defer ln.Close()
	fmt.Println("tcp echo server listening on :9000")

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Fatal(err)
		}
		go handle(conn)
	}
}

func handle(conn net.Conn) {
	defer conn.Close()
	fmt.Printf("connected: %s -> %s\n", conn.RemoteAddr(), conn.LocalAddr())

	buf := make([]byte, 1024)
	for {
		n, err := conn.Read(buf)
		if n > 0 {
			fmt.Printf("recv %d bytes: %q\n", n, buf[:n])
			if _, err := conn.Write(buf[:n]); err != nil {
				log.Println(err)
				return
			}
		}
		if err != nil {
			fmt.Printf("disconnected: %s (%v)\n", conn.RemoteAddr(), err)
			return
		}
	}
}
