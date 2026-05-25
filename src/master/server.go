package master

import (
	"bufio"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

func handleConnection(conn net.Conn, allowedNodes map[string]struct{}) {
	defer conn.Close()
	remote := conn.RemoteAddr().String()
	log.Printf("master: incoming connection from %s", remote)

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	msg, err := reader.ReadString('\n')
	if err != nil {
		return
	}

	msg = strings.TrimSpace(msg)

	if msg != "CENTRALISD" {
		log.Printf("master: %s bad hello %q", remote, msg)
		writer.WriteString("FAIL\n")
		writer.Flush()
		return
	}

	msg, err = reader.ReadString('\n')
	if err != nil {
		return
	}

	msg = strings.TrimSpace(msg)

	parts := strings.Split(msg, "|")
	if len(parts) != 2 {
		log.Printf("master: %s invalid id line %q", remote, msg)
		writer.WriteString("FAIL\n")
		writer.Flush()
		return
	}

	clientIDStr := parts[0]
	pubKeyStr := parts[1]

	pubKeyBytes, err := base64.RawURLEncoding.DecodeString(pubKeyStr)
	if err != nil {
		log.Printf("master: %s invalid pubkey b64: %v", remote, err)
		writer.WriteString("FAIL\n")
		writer.Flush()
		return
	}

	clientIDRaw, err := base64.RawURLEncoding.DecodeString(clientIDStr)
	if err != nil {
		log.Printf("master: %s invalid client id b64: %v", remote, err)
		writer.WriteString("FAIL\n")
		writer.Flush()
		return
	}

	sum := sha256.Sum256(pubKeyBytes)

	if !equal(sum[:], clientIDRaw) {
		log.Printf("master: %s client id mismatch id=%s", remote, clientIDStr)
		writer.WriteString("FAIL\n")
		writer.Flush()
		return
	}
	if len(allowedNodes) > 0 {
		if _, ok := allowedNodes[clientIDStr]; !ok {
			log.Printf("master: %s rejected: node not whitelisted id=%s", remote, clientIDStr)
			writer.WriteString("FAIL\n")
			writer.Flush()
			return
		}
	}

	pubKey := ed25519.PublicKey(pubKeyBytes)

	challenge := generateChallenge()

	writer.WriteString(challenge + "\n")
	writer.Flush()

	sigLine, err := reader.ReadString('\n')
	if err != nil {
		return
	}

	sigLine = strings.TrimSpace(sigLine)

	sig, err := base64.RawURLEncoding.DecodeString(sigLine)
	if err != nil {
		log.Printf("master: %s invalid signature b64: %v", remote, err)
		writer.WriteString("FAIL\n")
		writer.Flush()
		return
	}

	if ed25519.Verify(pubKey, []byte(challenge), sig) {
		log.Printf("master: %s auth ok id=%s", remote, clientIDStr)
		writer.WriteString("OK\n")
	} else {
		log.Printf("master: %s auth failed id=%s", remote, clientIDStr)
		writer.WriteString("FAIL\n")
	}

	writer.Flush()

	for {
		writer.WriteString("HEARTBEAT\n")
		writer.Flush()

		time.Sleep(time.Second * 10)
	}
}

func HostMasterServer(port int, allowedNodes map[string]struct{}) {
	server, err := net.Listen("tcp", ":"+strconv.Itoa(port))
	if err != nil {
		os.Exit(1)
	}
	log.Printf("master: tcp server listening on :%d", port)

	for {
		conn, err := server.Accept()
		if err != nil {
			log.Printf("master: accept error: %v", err)
			continue
		}

		go handleConnection(conn, allowedNodes)
	}
}

func generateChallenge() string {
	b := make([]byte, 64)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func equal(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
