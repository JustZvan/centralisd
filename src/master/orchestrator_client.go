package master

import (
	"bufio"
	"centralisd/src/core/config"
	"centralisd/src/core/protocol"
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
	"sync"
	"time"
)

type packetWriter struct {
	mu     sync.Mutex
	writer *bufio.Writer
}

func (w *packetWriter) writePacket(packet protocol.Packet) error {
	if w == nil || w.writer == nil {
		return io.ErrClosedPipe
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	return protocol.WritePacket(w.writer, packet)
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
	writer := &packetWriter{writer: bufio.NewWriter(conn)}

	if err := protocol.WriteHello(writer.writer, protocol.HeaderOrchestrator); err != nil {
		return err
	}

	// Orchestrator sends challenge, we respond with signed proof.
	challengePacket, err := protocol.ReadPacket(reader)
	if err != nil {
		return err
	}
	if challengePacket.Type != string(protocol.PacketAuthChallenge) {
		return fmt.Errorf("unexpected challenge packet type %q", challengePacket.Type)
	}
	challengePayload := protocol.AuthChallenge{}
	if err := protocol.DecodePayload(challengePacket, &challengePayload); err != nil {
		return err
	}

	pubKeyBytes, privKey, err := loadMasterKeys(cfg)
	if err != nil {
		return err
	}
	miAuth := buildMasterInfo(cfg)
	miAuth.PubKey = base64RawURL(pubKeyBytes)

	log.Println("our public key hash is: " + miAuth.PubKey)

	authHello := protocol.AuthHello{
		ID:        miAuth.ID,
		PubKey:    miAuth.PubKey,
		Role:      "master",
		Name:      miAuth.Name,
		Cluster:   miAuth.Cluster,
		Advertise: miAuth.Advertise,
	}
	authPacket, err := protocol.NewPacket(string(protocol.PacketAuthHello), authHello)
	if err != nil {
		return err
	}
	sig := ed25519.Sign(privKey, []byte(challengePayload.Challenge))
	proofPacket, err := protocol.NewPacket(string(protocol.PacketAuthProof), protocol.AuthProof{Signature: base64RawURL(sig)})
	if err != nil {
		return err
	}
	if err := writer.writePacket(authPacket); err != nil {
		return err
	}
	if err := writer.writePacket(proofPacket); err != nil {
		return err
	}
	authResp, err := protocol.ReadPacket(reader)
	if err != nil {
		return err
	}
	if authResp.Type == string(protocol.PacketError) {
		if strings.TrimSpace(authResp.Error) != "" {
			return fmt.Errorf("orchestrator auth failed: %s", strings.TrimSpace(authResp.Error))
		}
		return fmt.Errorf("orchestrator auth failed")
	}
	if authResp.Type != string(protocol.PacketAuthOK) {
		return fmt.Errorf("unexpected auth response %q", authResp.Type)
	}
	log.Printf("master: orchestrator auth ok")

	respCh := make(chan protocol.Packet)
	readErrCh := make(chan error, 1)
	go func() {
		readErrCh <- readOrchestratorLoop(reader, writer, respCh)
	}()

	mi := buildMasterInfo(cfg)
	mi.PubKey = "" // don’t keep retransmitting key after auth
	registerPacket, err := protocol.NewPacket(string(protocol.PacketMasterRegister), mi.MasterInfo)
	if err != nil {
		return err
	}
	if err := sendPacketAndExpectAuthOK(writer, respCh, readErrCh, registerPacket); err != nil {
		return err
	}
	log.Printf("master: orchestrator register ok")

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case err := <-readErrCh:
			if err == nil {
				return io.ErrUnexpectedEOF
			}
			return err
		case <-ticker.C:
			mi = buildMasterInfo(cfg)
			mi.PubKey = "" // don’t keep retransmitting key after auth
			heartbeatPacket, err := protocol.NewPacket(string(protocol.PacketMasterHeartbeat), mi.MasterInfo)
			if err != nil {
				return err
			}
			if err := sendPacketAndExpectAuthOK(writer, respCh, readErrCh, heartbeatPacket); err != nil {
				return err
			}
			log.Printf("master: orchestrator heartbeat ok")
		}
	}

	return nil
}

func sendPacketAndExpectAuthOK(writer *packetWriter, respCh <-chan protocol.Packet, readErrCh <-chan error, packet protocol.Packet) error {
	if err := writer.writePacket(packet); err != nil {
		return err
	}

	select {
	case resp := <-respCh:
		if resp.Type == string(protocol.PacketError) {
			if strings.TrimSpace(resp.Error) != "" {
				return fmt.Errorf("orchestrator error: %s", strings.TrimSpace(resp.Error))
			}
			return fmt.Errorf("orchestrator replied %q", resp.Type)
		}
		if resp.Type != string(protocol.PacketAuthOK) {
			return fmt.Errorf("orchestrator replied %q", resp.Type)
		}
		return nil
	case err := <-readErrCh:
		if err == nil {
			return io.ErrUnexpectedEOF
		}
		return err
	}
}

func readOrchestratorLoop(reader *bufio.Reader, writer *packetWriter, respCh chan<- protocol.Packet) error {
	for {
		packet, err := protocol.ReadPacket(reader)
		if err != nil {
			return err
		}
		if packet.Type == string(protocol.PacketAuthOK) || packet.Type == string(protocol.PacketError) {
			respCh <- packet
			continue
		}
		if packet.Type != string(protocol.PacketOrchCommand) {
			log.Printf("master: orchestrator message unknown type=%q", packet.Type)
			continue
		}
		cmd := protocol.OrchestratorCommand{}
		if err := protocol.DecodePayload(packet, &cmd); err != nil {
			log.Printf("master: orchestrator command invalid payload: %v", err)
			continue
		}
		if len(cmd.Command) == 0 {
			log.Printf("master: orchestrator command missing command node_id=%s", cmd.NodeID)
			continue
		}
		if cmd.NodeID == "" {
			replyPayload := handleMasterCommand(cmd.Command)
			reply, _ := protocol.NewReply(string(protocol.PacketOrchCommandReply), packet.ID, replyPayload)
			_ = writer.writePacket(reply)
			continue
		}
		cmdPacket, err := protocol.NewPacket(string(protocol.PacketNodeCommand), json.RawMessage(cmd.Command))
		if err != nil {
			log.Printf("master: orchestrator command packet failed: %v", err)
			continue
		}
		respPacket, err := sendCommandToNodeWait(cmd.NodeID, cmdPacket, 10*time.Second)
		if err != nil {
			reply, _ := protocol.NewReply(string(protocol.PacketOrchCommandReply), packet.ID, protocol.CommandReply{NodeID: cmd.NodeID, Status: "error", Message: err.Error()})
			_ = writer.writePacket(reply)
			continue
		}
		if respPacket.Type != string(protocol.PacketNodeCommandReply) {
			reply, _ := protocol.NewReply(string(protocol.PacketOrchCommandReply), packet.ID, protocol.CommandReply{NodeID: cmd.NodeID, Status: "error", Message: "unexpected reply type"})
			_ = writer.writePacket(reply)
			continue
		}
		replyPayload := protocol.CommandReply{}
		if err := protocol.DecodePayload(respPacket, &replyPayload); err != nil {
			reply, _ := protocol.NewReply(string(protocol.PacketOrchCommandReply), packet.ID, protocol.CommandReply{NodeID: cmd.NodeID, Status: "error", Message: "invalid reply payload"})
			_ = writer.writePacket(reply)
			continue
		}
		replyPayload.NodeID = cmd.NodeID
		reply, _ := protocol.NewReply(string(protocol.PacketOrchCommandReply), packet.ID, replyPayload)
		_ = writer.writePacket(reply)
	}
}

func handleMasterCommand(command json.RawMessage) protocol.CommandReply {
	cmd := protocol.NodeCommand{}
	if err := json.Unmarshal(command, &cmd); err != nil {
		return protocol.CommandReply{Status: "error", Message: "invalid command"}
	}

	switch cmd.Action {
	case "libvirt.domains.list":
		nodeIDs := listConnectedNodeIDs()
		log.Printf("master: aggregating vm list across %d connected nodes", len(nodeIDs))

		results := make([]protocol.VMListNode, 0, len(nodeIDs))
		for _, nodeID := range nodeIDs {
			if strings.TrimSpace(nodeID) == "" {
				continue
			}

			cmdPacket, err := protocol.NewPacket(string(protocol.PacketNodeCommand), json.RawMessage(command))
			if err != nil {
				return protocol.CommandReply{Status: "error", Message: "invalid command packet"}
			}

			respPacket, err := sendCommandToNodeWait(nodeID, cmdPacket, 10*time.Second)
			if err != nil {
				results = append(results, protocol.VMListNode{NodeID: nodeID, Error: err.Error()})
				continue
			}
			if respPacket.Type != string(protocol.PacketNodeCommandReply) {
				results = append(results, protocol.VMListNode{NodeID: nodeID, Error: "unexpected reply type"})
				continue
			}

			replyPayload := protocol.CommandReply{}
			if err := protocol.DecodePayload(respPacket, &replyPayload); err != nil {
				results = append(results, protocol.VMListNode{NodeID: nodeID, Error: "invalid reply payload"})
				continue
			}
			if replyPayload.Status != "ok" {
				results = append(results, protocol.VMListNode{NodeID: nodeID, Error: replyPayload.Message})
				continue
			}

			items := []protocol.VMDomain{}
			if err := json.Unmarshal(replyPayload.Output, &items); err != nil {
				results = append(results, protocol.VMListNode{NodeID: nodeID, Error: "invalid domain list"})
				continue
			}
			results = append(results, protocol.VMListNode{NodeID: nodeID, Domains: items})
		}

		payload, err := json.Marshal(protocol.VMListAggregate{Nodes: results})
		if err != nil {
			return protocol.CommandReply{Status: "error", Message: "failed to encode domain list"}
		}
		return protocol.CommandReply{Status: "ok", Output: payload}
	default:
		return protocol.CommandReply{Status: "error", Message: "node id required"}
	}
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

	nodes := make([]registry.NodeInfo, 0, 4)
	for _, nodeID := range listConnectedNodeIDs() {
		if strings.TrimSpace(nodeID) == "" {
			continue
		}
		nodes = append(nodes, registry.NodeInfo{ID: nodeID})
	}

	return registry.MasterInfo{MasterInfo: protocol.MasterInfo{
		ID:        id,
		Name:      name,
		Cluster:   cluster,
		Advertise: adv,
		PubKey:    "",
		Nodes:     nodes,
	}}
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
