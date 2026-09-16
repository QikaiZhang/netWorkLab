// tls/gencert: 生成自签发证书（测试用），输出 PEM 到 tls/certs/。
package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"log"
	"math/big"
	"net"
	"os"
	"time"
)

func main() {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		log.Fatal(err)
	}

	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "localhost"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{"localhost"},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
		IsCA:         true, // 自签发：自己给自己签名
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		log.Fatal(err)
	}

	if err := os.MkdirAll("tls/certs", 0o755); err != nil {
		log.Fatal(err)
	}
	certOut, err := os.Create("tls/certs/server.crt")
	if err != nil {
		log.Fatal(err)
	}
	pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: der})
	certOut.Close()

	keyOut, err := os.Create("tls/certs/server.key")
	if err != nil {
		log.Fatal(err)
	}
	derKey, _ := x509.MarshalECPrivateKey(key)
	pem.Encode(keyOut, &pem.Block{Type: "EC PRIVATE KEY", Bytes: derKey})
	keyOut.Close()

	log.Println("wrote tls/certs/server.crt and tls/certs/server.key")
}
