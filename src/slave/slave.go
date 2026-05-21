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
		return nil, fmt.Errorf("invalid PEM public key")
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
		return nil, fmt.Errorf("invalid PEM private key")
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
		fmt.Println("[-] connect:", err)
		os.Exit(1)
	}
	defer conn.Close()

	println("who")

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	pubKey, err := loadEd25519Public(cfg.Slave.PubKeyPath)
	if err != nil {
		fmt.Println("[-] pubkey load:", err)
		os.Exit(1)
	}

	privKey, err := loadEd25519Private(cfg.Slave.PrivKeyPath)
	if err != nil {
		fmt.Println("[-] privkey load:", err)
		os.Exit(1)
	}

	if _, err := writer.WriteString("CENTRALISD\n"); err != nil {
		fmt.Println("[-] write HELLO:", err)
		os.Exit(1)
	}
	if err := writer.Flush(); err != nil {
		fmt.Println("[-] flush HELLO:", err)
		os.Exit(1)
	}
	fmt.Println("[+] sent HELLO")

	sum := sha256.Sum256(pubKey)
	clientID := base64.RawURLEncoding.EncodeToString(sum[:])
	pubKeyStr := base64.RawURLEncoding.EncodeToString(pubKey)

	if _, err := writer.WriteString(clientID + "|" + pubKeyStr + "\n"); err != nil {
		fmt.Println("[-] write ID:", err)
		os.Exit(1)
	}
	if err := writer.Flush(); err != nil {
		fmt.Println("[-] flush ID:", err)
		os.Exit(1)
	}
	fmt.Println("[+] sent ID")

	challenge, err := reader.ReadString('\n')
	if err != nil {
		fmt.Println("[-] read challenge:", err)
		os.Exit(1)
	}
	challenge = strings.TrimSpace(challenge)

	if challenge == "FAIL" {
		fmt.Println("[-] server rejected handshake")
		os.Exit(1)
	}
	fmt.Println("[+] challenge:", challenge)

	sig := ed25519.Sign(privKey, []byte(challenge))
	sigStr := base64.RawURLEncoding.EncodeToString(sig)

	if _, err := writer.WriteString(sigStr + "\n"); err != nil {
		fmt.Println("[-] write sig:", err)
		os.Exit(1)
	}
	if err := writer.Flush(); err != nil {
		fmt.Println("[-] flush sig:", err)
		os.Exit(1)
	}
	fmt.Println("[+] sent signature")

	resp, err := reader.ReadString('\n')
	if err != nil {
		fmt.Println("[-] auth response:", err)
		os.Exit(1)
	}

	resp = strings.TrimSpace(resp)
	fmt.Println("[+] auth:", resp)

	if resp != "OK" {
		fmt.Println("[-] auth failed")
		return
	}

	fmt.Println("[+] connected")

	for {
		resp, err = reader.ReadString('\n')
		if err != nil {
			fmt.Println("[-] heartbeat read:", err)
			return
		}

		resp = strings.TrimSpace(resp)

		if resp == "HEARTBEAT" {
			fmt.Println("[+] heartbeat")

			hw := hardware.GetHardwareInfo()
			json_hw_b, err := json.Marshal(hw)
			if err != nil {
				fmt.Println("[-] hw marshal:", err)
				return
			}

			fmt.Println("[+] hw:", string(json_hw_b))
		} else {
			fmt.Println("[-] unexpected:", resp)
		}
	}
}
