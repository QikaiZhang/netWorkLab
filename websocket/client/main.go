// websocket/client: 连接 ws 服务，发一条文本消息、发一个 Ping，收 echo 与 Pong。
package main

import (
	"fmt"
	"log"
	"net"
	"time"

	"github.com/gorilla/websocket"
)

func main() {
	conn, _, err := websocket.DefaultDialer.Dial("ws://localhost:8084/ws", nil)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	// 发送一个 Ping 控制帧，server 的 gorilla 会自动回 Pong
	conn.SetPongHandler(func(appData string) error {
		fmt.Printf("pong received: %q\n", appData)
		return nil
	})

	if err := conn.WriteMessage(websocket.TextMessage, []byte("hello ws")); err != nil {
		log.Fatal(err)
	}
	if err := conn.WriteControl(websocket.PingMessage, []byte("keepalive"),
		time.Now().Add(time.Second)); err != nil {
		log.Fatal(err)
	}

	// 循环读消息（Pong 由 handler 消费，不会出现在这里）；超时视为结束
	conn.SetReadDeadline(time.Now().Add(1500 * time.Millisecond))
	for {
		mt, msg, err := conn.ReadMessage()
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				break
			}
			log.Fatal(err)
		}
		fmt.Printf("message: type=%d %q\n", mt, msg)
	}

	// 发送 Close 帧，走一次完整的 WebSocket 关闭握手
	if err := conn.WriteControl(websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, "bye"),
		time.Now().Add(time.Second)); err != nil {
		log.Fatal(err)
	}
}
