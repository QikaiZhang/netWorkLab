// http/minparser/server: 不用 net/http，直接在 TCP 上解析最小 HTTP 请求（GET/POST + Content-Length）。
package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"strconv"
	"strings"
)

func main() {
	ln, err := net.Listen("tcp", ":8082")
	if err != nil {
		log.Fatal(err)
	}
	defer ln.Close()
	fmt.Println("minimal http server listening on :8082")

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

	req, err := readRequest(conn)
	if err != nil {
		fmt.Println("bad request:", err)
		return
	}
	fmt.Printf("method=%s path=%s host=%q body=%q\n",
		req.method, req.path, req.header["host"], req.body)

	resp := fmt.Sprintf("you sent %s %s with %d body bytes\n",
		req.method, req.path, len(req.body))
	fmt.Fprintf(conn, "HTTP/1.1 200 OK\r\n"+
		"Content-Type: text/plain\r\n"+
		"Content-Length: %d\r\n"+
		"Connection: close\r\n"+
		"\r\n"+
		"%s", len(resp), resp)
}

type request struct {
	method, path string
	header       map[string]string
	body         []byte
}

func readRequest(conn net.Conn) (*request, error) {
	br := bufio.NewReader(conn)

	// 1. 请求行: METHOD PATH HTTP/1.1
	line, err := br.ReadString('\n')
	if err != nil {
		return nil, err
	}
	parts := strings.Fields(strings.TrimRight(line, "\r\n"))
	if len(parts) != 3 {
		return nil, fmt.Errorf("bad request line: %q", line)
	}
	req := &request{method: parts[0], path: parts[1], header: map[string]string{}}

	// 2. 头部: 逐行读直到空行
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break // 空行 = 头部结束
		}
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			return nil, fmt.Errorf("bad header: %q", line)
		}
		req.header[strings.ToLower(strings.TrimSpace(k))] = strings.TrimSpace(v)
	}

	// 3. Body: 按 Content-Length 精确读取
	if n, err := strconv.Atoi(req.header["content-length"]); err == nil && n > 0 {
		req.body = make([]byte, n)
		if _, err := io.ReadFull(br, req.body); err != nil {
			return nil, err
		}
	}
	return req, nil
}
