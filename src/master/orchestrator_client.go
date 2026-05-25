package master

import (
	"bufio"
	"centralisd/src/core/config"
	"centralisd/src/orchestrator/registry"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"
	"time"
)

type orchEnvelope struct {
	Type      string              `json:"type"`
	Master    registry.MasterInfo `json:"master"`
	Signature string              `json:"signature,omitempty"`
}

func ConnectToOrchestrator(cfg config.Config) {
	addr := strings.TrimSpace(cfg.Master.Orchestrator)
	if addr == "" {
		return
	}
	// Validate key config once; auth cannot work without it.
	if strings.TrimSpace(cfg.Master.PubKeyPath) == "" || strings.TrimSpace(cfg.Master.PrivKeyPath) == "" {
		log.Printf("master: orchestrator enabled but key paths missing (master.pubkeypath/master.privkeypath); not connecting")
		return
	}
	if _, _, err := loadMasterKeys(cfg); err != nil {
		log.Printf("master: failed to load master keys for orchestrator auth: %v; not connecting", err)
		return
	}

	log.Printf("master: connecting to orchestrator addr=%s cluster=%s advertise=%s", addr, strings.TrimSpace(cfg.Master.Cluster), strings.TrimSpace(cfg.Master.Advertise))

	backoff := 1 * time.Second
	for {
		if err := connectLoop(addr, cfg); err != nil {
			log.Printf("master: orchestrator connection error: %v; retrying in %s", err, backoff)
			time.Sleep(backoff)
			if backoff < 30*time.Second {
				backoff *= 2
				if backoff > 30*time.Second {
					backoff = 30 * time.Second
				}
			}
			continue
		}
		// connectLoop only returns nil if it was asked to stop (currently never).
		return
	}
}

func connectLoop(addr string, cfg config.Config) error {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return err
	}
	defer conn.Close()
	log.Printf("master: orchestrator connected remote=%s", conn.RemoteAddr().String())

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	if _, err := writer.WriteString("CENTRALISD-ORCH/1\n"); err != nil {
		return err
	}
	if err := writer.Flush(); err != nil {
		return err
	}

	// Orchestrator sends challenge, we respond with signed proof.
	challenge, err := reader.ReadString('\n')
	if err != nil {
		return err
	}
	challenge = strings.TrimSpace(challenge)

	pubKeyBytes, privKey, err := loadMasterKeys(cfg)
	if err != nil {
		return err
	}
	miAuth := buildMasterInfo(cfg)
	miAuth.PubKey = base64RawURL(pubKeyBytes)

	log.Println("our public key hash is: " + miAuth.PubKey)

	sig := ed25519.Sign(privKey, []byte(challenge))
	if err := sendAndExpectOK(reader, writer, orchEnvelope{Type: "master.auth", Master: miAuth, Signature: base64RawURL(sig)}); err != nil {
		return err
	}
	log.Printf("master: orchestrator auth ok")

	mi := buildMasterInfo(cfg)
	mi.PubKey = "" // don’t keep retransmitting key after auth
	if err := sendAndExpectOK(reader, writer, orchEnvelope{Type: "master.register", Master: mi}); err != nil {
		return err
	}
	log.Printf("master: orchestrator register ok")

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		mi = buildMasterInfo(cfg)
		mi.PubKey = "" // don’t keep retransmitting key after auth
		if err := sendAndExpectOK(reader, writer, orchEnvelope{Type: "master.heartbeat", Master: mi}); err != nil {
			return err
		}
		log.Printf("master: orchestrator heartbeat ok")
	}

	return nil
}

func sendAndExpectOK(reader *bufio.Reader, writer *bufio.Writer, env orchEnvelope) error {
	b, err := json.Marshal(env)
	if err != nil {
		return err
	}
	if _, err := writer.WriteString(string(b) + "\n"); err != nil {
		return err
	}
	if err := writer.Flush(); err != nil {
		return err
	}

	resp, err := reader.ReadString('\n')
	if err != nil {
		return err
	}
	resp = strings.TrimSpace(resp)
	if resp != "OK" {
		return fmt.Errorf("orchestrator replied %q", resp)
	}
	return nil
}

func buildMasterInfo(cfg config.Config) registry.MasterInfo {
	name := strings.TrimSpace(cfg.Master.Name)
	if name == "" {
		name, _ = os.Hostname()
	}
	cluster := strings.TrimSpace(cfg.Master.Cluster)
	if cluster == "" {
		cluster = "default"
	}
	adv := strings.TrimSpace(cfg.Master.Advertise)

	pubKeyBytes, _, _ := loadMasterKeys(cfg)
	id := ""
	if len(pubKeyBytes) > 0 {
		sum := sha256.Sum256(pubKeyBytes)
		id = base64RawURL(sum[:])
	}

	nodes := []registry.NodeInfo{{
		ID:   id,
		Name: name,
		IP:   strings.Join(getLocalIPs(), ","),
	}}

	return registry.MasterInfo{
		ID:        id,
		Name:      name,
		Cluster:   cluster,
		Advertise: adv,
		PubKey:    "",
		Nodes:     nodes,
	}
}

func loadMasterKeys(cfg config.Config) ([]byte, ed25519.PrivateKey, error) {
	pubPath := strings.TrimSpace(cfg.Master.PubKeyPath)
	privPath := strings.TrimSpace(cfg.Master.PrivKeyPath)
	if pubPath == "" || privPath == "" {
		return nil, nil, io.ErrUnexpectedEOF
	}
	pub, err := loadEd25519Public(pubPath)
	if err != nil {
		return nil, nil, err
	}
	priv, err := loadEd25519Private(privPath)
	if err != nil {
		return nil, nil, err
	}
	return []byte(pub), priv, nil
}

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

func base64RawURL(b []byte) string {
	return base64.RawURLEncoding.EncodeToString(b)
}

func getLocalIPs() []string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}

	out := make([]string, 0, 4)
	for _, iface := range ifaces {
		// Skip down and loopback.
		if iface.Flags&net.FlagUp == 0 {
			continue
		}
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			ip := addrToIP(a)
			if ip == "" {
				continue
			}
			out = append(out, ip)
		}
	}
	return out
}

func addrToIP(a net.Addr) string {
	switch v := a.(type) {
	case *net.IPNet:
		if v.IP == nil {
			return ""
		}
		ip4 := v.IP.To4()
		if ip4 == nil {
			return ""
		}
		return ip4.String()
	case *net.IPAddr:
		if v.IP == nil {
			return ""
		}
		ip4 := v.IP.To4()
		if ip4 == nil {
			return ""
		}
		return ip4.String()
	default:
		// Best-effort fallback for unexpected addr implementations.
		return fmt.Sprintf("%v", a)
	}
}
