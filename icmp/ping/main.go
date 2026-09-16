// icmp/ping: 发送 ICMP Echo Request 并等待 Echo Reply。
// 使用 SOCK_DGRAM + IPPROTO_ICMP（非特权 ICMP socket，macOS/BSD 无需 root）。
package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"syscall"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

func main() {
	host := "127.0.0.1"
	if len(os.Args) > 1 {
		host = os.Args[1]
	}
	ip, err := net.ResolveIPAddr("ip4", host)
	if err != nil {
		log.Fatal(err)
	}
	var dst [4]byte
	copy(dst[:], ip.IP.To4())

	// raw socket（"icmp4"）需要 root；datagram ICMP 是内核为 ping 开的非特权通道
	fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_DGRAM, syscall.IPPROTO_ICMP)
	if err != nil {
		log.Fatal(err)
	}
	defer syscall.Close(fd)

	// Echo Request: Type=8 Code=0；Reply 是 Type=0
	req := icmp.Message{
		Type: ipv4.ICMPTypeEcho, Code: 0,
		Body: &icmp.Echo{ID: 1, Seq: 1, Data: []byte("HELLO-ICMP")},
	}
	wire, err := req.Marshal(nil) // Marshal 会计算校验和
	if err != nil {
		log.Fatal(err)
	}

	start := time.Now()
	if err := syscall.Sendto(fd, wire, 0, &syscall.SockaddrInet4{Addr: dst}); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("sent Echo Request to %s (%d bytes)\n", ip, len(wire))

	syscall.SetsockoptTimeval(fd, syscall.SOL_SOCKET, syscall.SO_RCVTIMEO,
		&syscall.Timeval{Sec: 3})
	buf := make([]byte, 1500)
	n, _, err := syscall.Recvfrom(fd, buf, 0)
	if err != nil {
		log.Fatalf("no reply: %v", err)
	}
	rtt := time.Since(start)

	// 该 socket 收到的是含 IP 头的完整报文，按 IHL 跳过 IP 头再解析 ICMP
	ihl := int(buf[0]&0x0f) * 4
	reply, err := icmp.ParseMessage(1 /* IPv4 ICMP */, buf[ihl:n])
	if err != nil {
		log.Fatal(err)
	}
	if reply.Type == ipv4.ICMPTypeEchoReply {
		echo := reply.Body.(*icmp.Echo)
		fmt.Printf("got Echo Reply from %s: id=%d seq=%d data=%q rtt=%v\n",
			ip, echo.ID, echo.Seq, echo.Data, rtt)
		return
	}
	log.Fatalf("unexpected ICMP type: %v", reply.Type)
}
