// dns/lookup: 用标准库解析域名，观察 A/AAAA/CNAME 记录。
package main

import (
	"fmt"
	"log"
	"net"
	"os"
)

func main() {
	host := "example.com"
	if len(os.Args) > 1 {
		host = os.Args[1]
	}

	addrs, err := net.LookupHost(host) // 系统解析器（macOS 走 mDNSResponder）
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("LookupHost(%s):\n", host)
	for _, a := range addrs {
		ip := net.ParseIP(a)
		kind := "A (IPv4)"
		if ip.To4() == nil {
			kind = "AAAA (IPv6)"
		}
		fmt.Printf("  %-15s %s\n", a, kind)
	}

	cname, err := net.LookupCNAME(host)
	if err == nil {
		fmt.Printf("CNAME: %s\n", cname)
	}
}
