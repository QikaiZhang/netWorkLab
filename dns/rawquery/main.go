// dns/rawquery: 不用解析库，手工拼一个 DNS 查询报文发往 DNS 服务器，再手工解析响应。
package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"math/rand"
	"net"
	"os"
	"strings"
	"time"
)

func main() {
	domain := "example.com"
	if len(os.Args) > 1 {
		domain = os.Args[1]
	}
	server := "1.1.1.1:53" // 可换成你的本地 DNS
	if len(os.Args) > 2 {
		server = os.Args[2] + ":53"
	}

	conn, err := net.Dial("udp", server)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	query := buildQuery(domain, 1 /* type A */)
	fmt.Printf("query %d bytes -> %s\n", len(query), server)
	if _, err := conn.Write(query); err != nil {
		log.Fatal(err)
	}

	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	buf := make([]byte, 512)
	n, err := conn.Read(buf)
	if err != nil {
		log.Fatal(err)
	}
	parseResponse(buf[:n], domain)
}

// DNS 报文: Header(12B) | Question | (Answer...)
func buildQuery(domain string, qtype uint16) []byte {
	var b []byte
	// Header: ID, Flags(递归请求 RD=1), QDCOUNT=1, 其余为 0
	id := uint16(rand.Intn(1 << 16))
	b = binary.BigEndian.AppendUint16(b, id)
	b = binary.BigEndian.AppendUint16(b, 0x0100) // RD=1
	b = binary.BigEndian.AppendUint16(b, 1)      // QDCOUNT
	b = binary.BigEndian.AppendUint16(b, 0)      // ANCOUNT
	b = binary.BigEndian.AppendUint16(b, 0)      // NSCOUNT
	b = binary.BigEndian.AppendUint16(b, 0)      // ARCOUNT

	// Question: QNAME(每个 label 前置长度字节), QTYPE, QCLASS
	for _, label := range strings.Split(domain, ".") {
		b = append(b, byte(len(label)))
		b = append(b, label...)
	}
	b = append(b, 0) // QNAME 以 0 长度 label 结束
	b = binary.BigEndian.AppendUint16(b, qtype)
	b = binary.BigEndian.AppendUint16(b, 1) // IN
	return b
}

func parseResponse(buf []byte, domain string) {
	id := binary.BigEndian.Uint16(buf[0:2])
	flags := binary.BigEndian.Uint16(buf[2:4])
	qd, an := binary.BigEndian.Uint16(buf[4:6]), binary.BigEndian.Uint16(buf[6:8])
	fmt.Printf("response %d bytes: id=%d rcode=%d questions=%d answers=%d\n",
		len(buf), id, flags&0xF, qd, an)

	// 跳过 Question 区（解析 QNAME 到 0 字节，再加 4 字节 QTYPE/QCLASS）
	pos := 12
	for buf[pos] != 0 {
		pos += int(buf[pos]) + 1
	}
	pos += 5

	for i := 0; i < int(an); i++ {
		name, next := readName(buf, pos)
		pos = next
		rtype := binary.BigEndian.Uint16(buf[pos : pos+2])
		ttl := binary.BigEndian.Uint32(buf[pos+4 : pos+8])
		rdlen := int(binary.BigEndian.Uint16(buf[pos+8 : pos+10]))
		rdata := buf[pos+10 : pos+10+rdlen]
		pos += 10 + rdlen

		value := name
		if rtype == 1 && rdlen == 4 { // A 记录: 4 字节 IPv4
			value = net.IP(rdata).String()
		}
		fmt.Printf("  answer: type=%d ttl=%d data=%s\n", rtype, ttl, value)
	}
}

// 域名可能被压缩（0xC0 开头的指针），返回 (名字, 下一个位置)
func readName(buf []byte, pos int) (string, int) {
	var labels []string
	p := pos
	for {
		l := int(buf[p])
		if l == 0 {
			p++
			break
		}
		if l&0xC0 == 0xC0 { // 压缩指针: 指向报文中之前出现过的名字
			offset := int(binary.BigEndian.Uint16(buf[p:p+2]) & 0x3FFF)
			name, _ := readName(buf, offset)
			labels = append(labels, name)
			p += 2
			return strings.Join(labels, "."), p
		}
		labels = append(labels, string(buf[p+1:p+1+l]))
		p += 1 + l
	}
	return strings.Join(labels, "."), p
}
