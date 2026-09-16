// tcp/echo/client: 连接本机 9000 端口，发送一行文本并读取回显。
package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"time"
)

func main() {
	conn, err := net.Dial("tcp", "localhost:9000")
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()
	fmt.Printf("local addr: %s\n", conn.LocalAddr())

	msg := fmt.Sprintf("hello, tcp (t=%s)", time.Now().Format("15:04:05.000"))
	if _, err := conn.Write([]byte(msg + "\n")); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("sent: %q\n", msg+"\n")

	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil && err != io.EOF {
		log.Fatal(err)
	}
	fmt.Printf("echo: %q\n", string(buf[:n]))
}
