// websocket/server: /ws 提供 WebSocket echo 服务（Upgrade 由 gorilla 完成），首页附浏览器演示。
package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{}

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><body>
<h3>WebSocket demo</h3>
<script>
  const ws = new WebSocket('ws://' + location.host + '/ws');
  ws.onopen = () => ws.send('hello from browser');
  ws.onmessage = e => document.body.append(e.data + String.fromCharCode(10));
</script>
</body></html>`)
	})

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil) // 校验 Upgrade 头并回复 101
		if err != nil {
			log.Println("upgrade:", err)
			return
		}
		defer conn.Close()
		fmt.Printf("ws connected: %s\n", conn.RemoteAddr())

		for {
			mt, msg, err := conn.ReadMessage()
			if err != nil {
				fmt.Printf("ws closed: %v\n", err)
				return
			}
			fmt.Printf("frame: type=%d len=%d %q\n", mt, len(msg), msg)
			if err := conn.WriteMessage(mt, append([]byte("echo: "), msg...)); err != nil {
				return
			}
		}
	})

	log.Fatal(http.ListenAndServe(":8084", nil))
}
