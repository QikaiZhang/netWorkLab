// tls/server: 在 :8443 上提供 HTTPS（HTTP over TLS）。
package main

import (
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// r.TLS 非空说明这条连接跑在 TLS 上，版本由握手协商决定
		fmt.Fprintf(w, "hello over %s, cipher=%s\n",
			tls.VersionName(r.TLS.Version),
			tls.CipherSuiteName(r.TLS.CipherSuite))
	})

	srv := &http.Server{
		Addr:    ":8443",
		Handler: mux,
		TLSConfig: &tls.Config{
			MinVersion: tls.VersionTLS12, // 禁用不安全的旧版本
		},
	}
	log.Fatal(srv.ListenAndServeTLS("tls/certs/server.crt", "tls/certs/server.key"))
}
