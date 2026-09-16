// tcp/stream/server: 打印每一次 Read 实际读到的字节数，观察 TCP 是否保留写入边界。
package main

import (
	"fmt"
	"log"
	"net"
	"sync/atomic"
)

func main() {
	ln, err := net.Listen("tcp", ":9001")
	if err != nil {
		log.Fatal(err)
	}
	defer ln.Close()
	fmt.Println("byte-stream server listening on :9001")

	var id atomic.Int64
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Fatal(err)
		}
		go func(c net.Conn) {
			defer c.Close()
			cid := id.Add(1)
			fmt.Printf("[conn %d] connected from %s\n", cid, c.RemoteAddr())

			buf := make([]byte, 1024)
			for {
				n, err := c.Read(buf)
				fmt.Printf("[conn %d] Read() -> %d bytes: %q\n", cid, n, buf[:n])
				if err != nil {
					fmt.Printf("[conn %d] closed (%v)\n", cid, err)
					return
				}
			}
		}(conn)
	}
}
