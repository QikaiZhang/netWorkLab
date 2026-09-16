// sse/server: 用 net/http 实现 text/event-stream 持续推送，并附一个 EventSource 演示页。
package main

import (
	"fmt"
	"log"
	"net/http"
	"time"
)

const indexHTML = `<!doctype html>
<html><body>
<h3>SSE demo (EventSource)</h3>
<ul id="log"></ul>
<script>
  const log = document.getElementById('log');
  const es = new EventSource('/events');          // 浏览器自动发起 GET /events
  es.onmessage = e => {                           // 自动重连由浏览器完成
    const li = document.createElement('li');
    li.textContent = e.data;
    log.append(li);
  };
</script>
</body></html>`

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, indexHTML)
	})

	http.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")

		fmt.Fprintf(w, "retry: 3000\n\n") // 告诉浏览器：断线后 3 秒重连
		flusher.Flush()
		for i := 1; ; i++ {
			fmt.Fprintf(w, "id: %d\ndata: message %d\n\n", i, i)
			flusher.Flush() // 立刻发出去，不等缓冲区攒满
			time.Sleep(time.Second)
		}
	})

	log.Fatal(http.ListenAndServe(":8083", nil))
}
