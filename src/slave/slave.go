package slave

import (
	"bufio"
	"centralisd/src/core/config"
	"centralisd/src/slave/hardware"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
)

func loadEd25519Public(path string) (ed25519.PublicKey, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	block, _ := pem.Decode(b)
	if block == nil {
		return nil, fmt.Errorf("invalid pem public key")
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}

	key, ok := pub.(ed25519.PublicKey)
	if !ok {
		return nil, fmt.Errorf("not ed25519 public key")
	}

	return key, nil
}

func loadEd25519Private(path string) (ed25519.PrivateKey, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	block, _ := pem.Decode(b)
	if block == nil {
		return nil, fmt.Errorf("invalid pem private key")
	}

	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}

	edKey, ok := key.(ed25519.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("not an ed25519 private key")
	}

	return edKey, nil
}

func Connect(addr string, cfg config.Config) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		log.Printf("slave: connect: %v", err)
		os.Exit(1)
	}
	defer conn.Close()

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	pubKey, err := loadEd25519Public(cfg.Slave.PubKeyPath)
	if err != nil {
		log.Printf("slave: pubkey load: %v", err)
		os.Exit(1)
	}

	privKey, err := loadEd25519Private(cfg.Slave.PrivKeyPath)
	if err != nil {
		log.Printf("slave: privkey load: %v", err)
		os.Exit(1)
	}

	if _, err := writer.WriteString("CENTRALISD\n"); err != nil {
		log.Printf("slave: write hello: %v", err)
		os.Exit(1)
	}
	if err := writer.Flush(); err != nil {
		log.Printf("slave: flush hello: %v", err)
		os.Exit(1)
	}
	log.Printf("slave: sent hello")

	sum := sha256.Sum256(pubKey)
	clientID := base64.RawURLEncoding.EncodeToString(sum[:])
	pubKeyStr := base64.RawURLEncoding.EncodeToString(pubKey)

	if _, err := writer.WriteString(clientID + "|" + pubKeyStr + "\n"); err != nil {
		log.Printf("slave: write id: %v", err)
		os.Exit(1)
	}
	if err := writer.Flush(); err != nil {
		log.Printf("slave: flush id: %v", err)
		os.Exit(1)
	}
	log.Printf("slave: sent id")

	challenge, err := reader.ReadString('\n')
	if err != nil {
		log.Printf("slave: read challenge: %v", err)
		os.Exit(1)
	}
	challenge = strings.TrimSpace(challenge)

	if challenge == "FAIL" {
		log.Printf("slave: server rejected handshake")
		os.Exit(1)
	}
	log.Printf("slave: challenge: %s", challenge)

	sig := ed25519.Sign(privKey, []byte(challenge))
	sigStr := base64.RawURLEncoding.EncodeToString(sig)

	if _, err := writer.WriteString(sigStr + "\n"); err != nil {
		log.Printf("slave: write signature: %v", err)
		os.Exit(1)
	}
	if err := writer.Flush(); err != nil {
		log.Printf("slave: flush signature: %v", err)
		os.Exit(1)
	}
	log.Printf("slave: sent signature")

	resp, err := reader.ReadString('\n')
	if err != nil {
		log.Printf("slave: auth response: %v", err)
		os.Exit(1)
	}

	resp = strings.TrimSpace(resp)
	log.Printf("slave: auth: %s", resp)

	if resp != "OK" {
		log.Printf("slave: auth failed")
		return
	}

	log.Printf("slave: connected")

	for {
		resp, err = reader.ReadString('\n')
		if err != nil {
			log.Printf("slave: heartbeat read: %v", err)
			return
		}

		resp = strings.TrimSpace(resp)

		if resp == "HEARTBEAT" {
			log.Printf("slave: heartbeat")

			hw := hardware.GetHardwareInfo()
			json_hw_b, err := json.Marshal(hw)
			if err != nil {
				log.Printf("slave: hw marshal: %v", err)
				return
			}
			log.Printf("slave: hw: %s", string(json_hw_b))
		} else {
			log.Printf("slave: unexpected: %s", resp)
		}
	}
}
