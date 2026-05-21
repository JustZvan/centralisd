package master

import (
	"bufio"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

func handleConnection(conn net.Conn) {
	defer conn.Close()

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	msg, err := reader.ReadString('\n')
	if err != nil {
		return
	}

	msg = strings.TrimSpace(msg)

	if msg != "CENTRALISD" {
		println("hoi")
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
		writer.WriteString("FAIL\n")
		writer.Flush()
		return
	}

	clientIDStr := parts[0]
	pubKeyStr := parts[1]

	pubKeyBytes, err := base64.RawURLEncoding.DecodeString(pubKeyStr)
	if err != nil {
		writer.WriteString("FAIL\n")
		writer.Flush()
		return
	}

	clientIDRaw, err := base64.RawURLEncoding.DecodeString(clientIDStr)
	if err != nil {
		writer.WriteString("FAIL\n")
		writer.Flush()
		return
	}

	sum := sha256.Sum256(pubKeyBytes)

	if !equal(sum[:], clientIDRaw) {
		writer.WriteString("FAIL\n")
		writer.Flush()
		return
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
		writer.WriteString("FAIL\n")
		writer.Flush()
		return
	}

	if ed25519.Verify(pubKey, []byte(challenge), sig) {
		writer.WriteString("OK\n")
	} else {
		writer.WriteString("FAIL\n")
	}

	writer.Flush()

	for {
		writer.WriteString("HEARTBEAT\n")
		writer.Flush()

		time.Sleep(time.Second * 10)
	}
}

func HostMasterServer(port int) {
	server, err := net.Listen("tcp", ":"+strconv.Itoa(port))
	if err != nil {
		os.Exit(1)
	}

	println("[+] Started server!")

	for {
		conn, err := server.Accept()
		if err != nil {
			println("[-] accept error:", err.Error())
			continue
		}

		go handleConnection(conn)
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
