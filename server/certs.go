package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ensureCert erzeugt beim ersten Start ein eigenes (selbstsigniertes) Zertifikat.
// Die Agents prüfen den Fingerabdruck dieses Zertifikats – so ist die Verbindung
// auch ohne gekauftes Zertifikat gegen Mitlesen und Umleiten geschützt.
func ensureCert(dir string) (certPath, keyPath, fingerprint string, err error) {
	certPath = filepath.Join(dir, "tls-cert.pem")
	keyPath = filepath.Join(dir, "tls-key.pem")
	if b, e := os.ReadFile(certPath); e == nil {
		if _, e2 := os.Stat(keyPath); e2 == nil {
			blk, _ := pem.Decode(b)
			if blk == nil {
				return "", "", "", errors.New("tls-cert.pem ist ungültig")
			}
			sum := sha256.Sum256(blk.Bytes)
			return certPath, keyPath, hex.EncodeToString(sum[:]), nil
		}
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return
	}
	serial, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 127))
	host, _ := os.Hostname()
	tmpl := x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: "Nicolaj Fernwartung", Organization: []string{"Nicolaj Webdesign"}},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{"localhost"},
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")},
	}
	if host != "" {
		tmpl.DNSNames = append(tmpl.DNSNames, host, strings.ToLower(host)+".local")
	}
	for _, ip := range localIPv4s() {
		tmpl.IPAddresses = append(tmpl.IPAddresses, net.ParseIP(ip))
	}
	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &key.PublicKey, key)
	if err != nil {
		return
	}
	kb, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return
	}
	if err = os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: kb}), 0o600); err != nil {
		return
	}
	if err = os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o644); err != nil {
		return
	}
	sum := sha256.Sum256(der)
	return certPath, keyPath, hex.EncodeToString(sum[:]), nil
}

func localIPv4s() []string {
	var out []string
	ifaces, _ := net.Interfaces()
	for _, ifc := range ifaces {
		if ifc.Flags&net.FlagUp == 0 || ifc.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, _ := ifc.Addrs()
		for _, ad := range addrs {
			ipn, ok := ad.(*net.IPNet)
			if !ok {
				continue
			}
			ip := ipn.IP.To4()
			if ip == nil || ip.IsLinkLocalUnicast() {
				continue
			}
			out = append(out, ip.String())
		}
	}
	return out
}

func fmtFingerprint(h string) string {
	h = strings.ToUpper(h)
	var parts []string
	for i := 0; i+2 <= len(h); i += 2 {
		parts = append(parts, h[i:i+2])
	}
	return strings.Join(parts, ":")
}
