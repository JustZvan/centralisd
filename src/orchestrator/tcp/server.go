package tcp

import (
	"bufio"
	"centralisd/src/core/protocol"
	"centralisd/src/orchestrator/registry"
	"encoding/json"
	"errors"
	"log"
	"net"
	"sync"
	"time"
)

type envelope struct {
	Type   string               `json:"type"`
	Master *registry.MasterInfo `json:"master,omitempty"`
	// Signature is base64url(raw(ed25519.Sign(privKey, challenge)))
	Signature string `json:"signature,omitempty"`
}

type connWriter struct {
	mu     sync.Mutex
	writer *bufio.Writer
}

func (w *connWriter) writeLine(line string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return protocol.WriteLine(w.writer, line)
}

type masterConn struct {
	id     string
	writer *connWriter
	reader *bufio.Reader
	pendMu sync.Mutex
	pend   map[string]chan protocol.Packet
}

func (m *masterConn) sendRequest(packet protocol.Packet, timeout time.Duration) (protocol.Packet, error) {
	if m == nil {
		return protocol.Packet{}, errors.New("nil master conn")
	}
	if packet.ID == "" {
		packet.ID = protocol.NewID()
	}
	respCh := make(chan protocol.Packet, 1)
	m.pendMu.Lock()
	if m.pend == nil {
		m.pend = map[string]chan protocol.Packet{}
	}
	m.pend[packet.ID] = respCh
	m.pendMu.Unlock()

	m.writer.mu.Lock()
	if err := protocol.WritePacket(m.writer.writer, packet); err != nil {
		m.writer.mu.Unlock()
		return protocol.Packet{}, err
	}
	if err := m.writer.writer.Flush(); err != nil {
		m.writer.mu.Unlock()
		return protocol.Packet{}, err
	}
	m.writer.mu.Unlock()

	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	select {
	case resp := <-respCh:
		return resp, nil
	case <-time.After(timeout):
		m.pendMu.Lock()
		delete(m.pend, packet.ID)
		m.pendMu.Unlock()
		return protocol.Packet{}, errors.New("timeout waiting for reply")
	}
}

func (m *masterConn) resolveReply(packet protocol.Packet) bool {
	if packet.ReplyTo == "" {
		return false
	}
	m.pendMu.Lock()
	ch := m.pend[packet.ReplyTo]
	if ch != nil {
		delete(m.pend, packet.ReplyTo)
	}
	m.pendMu.Unlock()
	if ch == nil {
		return false
	}
	ch <- packet
	return true
}

var masterConns = struct {
	mu    sync.RWMutex
	conns map[string]*masterConn
}{conns: map[string]*masterConn{}}

func registerMasterConn(id string, writer *connWriter, reader *bufio.Reader) *masterConn {
	if id == "" || writer == nil || reader == nil {
		return nil
	}
	mc := &masterConn{id: id, writer: writer, reader: reader, pend: map[string]chan protocol.Packet{}}
	masterConns.mu.Lock()
	masterConns.conns[id] = mc
	masterConns.mu.Unlock()
	return mc
}

func unregisterMasterConn(id string) {
	if id == "" {
		return
	}
	masterConns.mu.Lock()
	delete(masterConns.conns, id)
	masterConns.mu.Unlock()
}

func sendCommandWait(masterID string, cmd protocol.OrchestratorCommand, timeout time.Duration) (protocol.CommandReply, error) {
	if masterID == "" {
		return protocol.CommandReply{}, errors.New("master id is empty")
	}
	masterConns.mu.RLock()
	mc := masterConns.conns[masterID]
	masterConns.mu.RUnlock()
	if mc == nil {
		return protocol.CommandReply{}, errors.New("master not connected")
	}

	target := "master"
	if cmd.NodeID != "" {
		target = "node=" + cmd.NodeID
	}
	log.Printf("orchestrator: sending command master_id=%s target=%s", masterID, target)

	packet, err := protocol.NewPacket(string(protocol.PacketOrchCommand), cmd)
	if err != nil {
		return protocol.CommandReply{}, err
	}
	if packet.ID == "" {
		packet.ID = protocol.NewID()
	}

	respPacket, err := mc.sendRequest(packet, timeout)
	if err != nil {
		return protocol.CommandReply{}, err
	}
	if respPacket.Type != string(protocol.PacketOrchCommandReply) {
		return protocol.CommandReply{}, errors.New("unexpected reply type")
	}
	if respPacket.ReplyTo != packet.ID {
		return protocol.CommandReply{}, errors.New("reply id mismatch")
	}
	reply := protocol.CommandReply{}
	if err := protocol.DecodePayload(respPacket, &reply); err != nil {
		return protocol.CommandReply{}, errors.New("invalid reply payload")
	}
	return reply, nil
}

func SendCommandWait(masterID, nodeID string, command json.RawMessage, timeout time.Duration) (protocol.CommandReply, error) {
	if nodeID == "" {
		return protocol.CommandReply{}, errors.New("node id is empty")
	}
	return sendCommandWait(masterID, protocol.OrchestratorCommand{NodeID: nodeID, Command: command}, timeout)
}

func SendMasterCommandWait(masterID string, command json.RawMessage, timeout time.Duration) (protocol.CommandReply, error) {
	return sendCommandWait(masterID, protocol.OrchestratorCommand{Command: command}, timeout)
}

func SendCommand(masterID, nodeID string, command json.RawMessage) error {
	if masterID == "" {
		return errors.New("master id is empty")
	}
	if nodeID == "" {
		return errors.New("node id is empty")
	}
	masterConns.mu.RLock()
	mc := masterConns.conns[masterID]
	masterConns.mu.RUnlock()
	if mc == nil {
		return errors.New("master not connected")
	}
	packet, err := protocol.NewPacket(string(protocol.PacketOrchCommand), protocol.OrchestratorCommand{NodeID: nodeID, Command: command})
	if err != nil {
		return err
	}
	return mc.writer.writeLine(mustJSON(packet))
}

func HostOrchestratorTCPServer(listenAddr string, store *registry.Store, masterWhitelist map[string]map[string]struct{}) error {
	if listenAddr == "" {
		listenAddr = ":49150"
	}
	if store == nil {
		return errors.New("nil store")
	}

	ln, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return err
	}
	log.Printf("orchestrator: tcp registry listening on %s", listenAddr)

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("orchestrator: accept error: %v", err)
			continue
		}
		go handleConn(conn, store, masterWhitelist)
	}
}

func handleConn(conn net.Conn, store *registry.Store, masterWhitelist map[string]map[string]struct{}) {
	defer conn.Close()
	remote := conn.RemoteAddr().String()
	log.Printf("orchestrator: connection from %s", remote)

	_ = conn.SetDeadline(time.Now().Add(30 * time.Second))

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)
	connWriter := &connWriter{writer: writer}

	if err := protocol.ReadHello(reader, protocol.HeaderOrchestrator); err != nil {
		log.Printf("orchestrator: %s bad hello: %v", remote, err)
		_ = connWriter.writeLine(mustJSON(protocol.NewError("invalid hello")))
		return
	}

	// After hello, allow long-lived connections.
	_ = conn.SetDeadline(time.Time{})

	// Challenge-response: client must prove it owns the master key.
	challenge := protocol.GenerateChallenge()
	challengePacket, err := protocol.NewPacket(string(protocol.PacketAuthChallenge), protocol.AuthChallenge{Challenge: challenge})
	if err != nil {
		_ = connWriter.writeLine(mustJSON(protocol.NewError("challenge error")))
		return
	}
	_ = connWriter.writeLine(mustJSON(challengePacket))

	authPacket, err := protocol.ReadPacket(reader)
	if err != nil {
		log.Printf("orchestrator: %s read auth response: %v", remote, err)
		return
	}
	if authPacket.Type != string(protocol.PacketAuthHello) {
		log.Printf("orchestrator: %s auth wrong type=%q", remote, authPacket.Type)
		_ = connWriter.writeLine(mustJSON(protocol.NewError("auth wrong type")))
		return
	}
	authHello := protocol.AuthHello{}
	if err := protocol.DecodePayload(authPacket, &authHello); err != nil {
		log.Printf("orchestrator: %s auth payload invalid: %v", remote, err)
		_ = connWriter.writeLine(mustJSON(protocol.NewError("auth payload invalid")))
		return
	}
	if authHello.Role != "master" {
		log.Printf("orchestrator: %s auth wrong role=%q", remote, authHello.Role)
		_ = connWriter.writeLine(mustJSON(protocol.NewError("auth wrong role")))
		return
	}
	proofPacket, err := protocol.ReadPacket(reader)
	if err != nil {
		log.Printf("orchestrator: %s auth proof read: %v", remote, err)
		return
	}
	if proofPacket.Type != string(protocol.PacketAuthProof) {
		log.Printf("orchestrator: %s auth proof wrong type=%q", remote, proofPacket.Type)
		_ = connWriter.writeLine(mustJSON(protocol.NewError("auth proof wrong type")))
		return
	}
	proof := protocol.AuthProof{}
	if err := protocol.DecodePayload(proofPacket, &proof); err != nil {
		log.Printf("orchestrator: %s auth proof invalid: %v", remote, err)
		_ = connWriter.writeLine(mustJSON(protocol.NewError("auth proof invalid")))
		return
	}
	masterInfo := registry.MasterInfo{MasterInfo: protocol.MasterInfo{
		ID:        authHello.ID,
		Name:      authHello.Name,
		Cluster:   authHello.Cluster,
		Advertise: authHello.Advertise,
		PubKey:    authHello.PubKey,
	}}
	if !registry.VerifyMasterAuth(challenge, masterInfo, proof.Signature) {
		log.Printf("orchestrator: %s auth verify failed master_id=%s cluster=%s", remote, authHello.ID, authHello.Cluster)
		_ = connWriter.writeLine(mustJSON(protocol.NewError("auth verify failed")))
		return
	}
	if len(masterWhitelist) > 0 {
		clusterSet, ok := masterWhitelist[authHello.Cluster]
		if !ok {
			log.Printf("orchestrator: %s auth rejected: cluster not whitelisted cluster=%s master_id=%s", remote, authHello.Cluster, authHello.ID)
			_ = connWriter.writeLine(mustJSON(protocol.NewError("cluster not allowed")))
			return
		}
		if _, ok := clusterSet[authHello.ID]; !ok {
			log.Printf("orchestrator: %s auth rejected: master id not whitelisted cluster=%s master_id=%s", remote, authHello.Cluster, authHello.ID)
			_ = connWriter.writeLine(mustJSON(protocol.NewError("master not allowed")))
			return
		}
	}
	authOK, _ := protocol.NewPacket(string(protocol.PacketAuthOK), nil)
	_ = connWriter.writeLine(mustJSON(authOK))
	log.Printf("orchestrator: %s auth ok cluster=%s master_id=%s", remote, authHello.Cluster, authHello.ID)

	// Pin the authenticated master ID for this connection.
	authedMasterID := authHello.ID
	authedClusterID := authHello.Cluster
	mc := registerMasterConn(authedMasterID, connWriter, reader)
	defer unregisterMasterConn(authedMasterID)

	for {
		packet, err := protocol.ReadPacket(reader)
		if err != nil {
			log.Printf("orchestrator: %s read error: %v", remote, err)
			return
		}
		if mc != nil && mc.resolveReply(packet) {
			continue
		}
		switch packet.Type {
		case string(protocol.PacketMasterRegister), string(protocol.PacketMasterHeartbeat):
			mi := protocol.MasterInfo{}
			if err := protocol.DecodePayload(packet, &mi); err != nil {
				_ = connWriter.writeLine(mustJSON(protocol.NewError("payload invalid")))
				continue
			}
			if mi.ID == "" {
				_ = connWriter.writeLine(mustJSON(protocol.NewError("master id missing")))
				continue
			}
			if mi.ID != authedMasterID {
				log.Printf("orchestrator: %s rejected: master id changed authed=%s got=%s", remote, authedMasterID, mi.ID)
				_ = connWriter.writeLine(mustJSON(protocol.NewError("master id changed")))
				continue
			}
			if mi.Cluster != authedClusterID {
				log.Printf("orchestrator: %s rejected: cluster changed authed=%s got=%s", remote, authedClusterID, mi.Cluster)
				_ = connWriter.writeLine(mustJSON(protocol.NewError("cluster changed")))
				continue
			}
			if store.UpsertMaster(registry.MasterInfo{MasterInfo: mi}) {
				okPacket, _ := protocol.NewPacket(string(protocol.PacketAuthOK), nil)
				_ = connWriter.writeLine(mustJSON(okPacket))
				if packet.Type == string(protocol.PacketMasterRegister) {
					log.Printf("orchestrator: %s registered master_id=%s cluster=%s", remote, mi.ID, mi.Cluster)
				}
			} else {
				log.Printf("orchestrator: %s rejected update type=%s master_id=%s cluster=%s", remote, packet.Type, mi.ID, mi.Cluster)
				_ = connWriter.writeLine(mustJSON(protocol.NewError("update rejected")))
			}
		default:
			log.Printf("orchestrator: %s unknown message type=%q", remote, packet.Type)
			_ = connWriter.writeLine(mustJSON(protocol.NewError("unknown message type")))
		}
	}
}

func mustJSON(p protocol.Packet) string {
	b, _ := json.Marshal(p)
	return string(b)
}
