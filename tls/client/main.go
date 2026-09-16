// tls/client: 校验自签证书后连接 HTTPS 服务器，打印协商结果与响应。
package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
)

func main() {
	certPEM, err := os.ReadFile("tls/certs/server.crt")
	if err != nil {
		log.Fatal(err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(certPEM) {
		log.Fatal("failed to parse cert")
	}

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				// 用自签证书作为信任根（生产环境用系统根证书，而不是关掉校验）
				RootCAs:    pool,
				ServerName: "localhost",
				MinVersion: tls.VersionTLS12,
			},
		},
	}

	resp, err := client.Get("https://localhost:8443/")
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	cs := resp.TLS
	fmt.Printf("negotiated: %s, cipher=%s\n",
		tls.VersionName(cs.Version), tls.CipherSuiteName(cs.CipherSuite))
	fmt.Printf("server cert CN=%s DNSNames=%v\n",
		cs.PeerCertificates[0].Subject.CommonName, cs.PeerCertificates[0].DNSNames)

	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("body: %s", body)
}
