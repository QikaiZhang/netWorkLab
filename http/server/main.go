// http/server: 用 net/http 提供 GET /hello 和 POST /users。
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("/hello", func(w http.ResponseWriter, r *http.Request) {
		// HandlerFunc 收到请求时，TCP 连接、HTTP 解析都已由标准库完成
		fmt.Printf("%s %s %s from %s\n", r.Method, r.URL.Path, r.Proto, r.RemoteAddr)
		fmt.Fprintf(w, "hello, this is http over tcp (proto=%s)\n", r.Proto)
	})

	mux.HandleFunc("/users", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"id": 1, "name": req.Name})
	})

	log.Fatal(http.ListenAndServe(":8081", mux))
}
