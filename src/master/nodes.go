package master

import (
	"bufio"
	"centralisd/src/core/protocol"
	"errors"
	"log"
	"net"
	"sync"
	"time"
)

type nodeConn struct {
	id     string
	conn   net.Conn
	reader *bufio.Reader
	writer *bufio.Writer
	mu     sync.Mutex
	pendMu sync.Mutex
	pend   map[string]chan protocol.Packet
}

func (n *nodeConn) writePacket(p protocol.Packet) error {
	if n == nil {
		return errors.New("nil node")
	}
	if p.ID == "" {
		p.ID = protocol.NewID()
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	return protocol.WritePacket(n.writer, p)
}

func (n *nodeConn) sendRequest(p protocol.Packet, timeout time.Duration) (protocol.Packet, error) {
	if n == nil {
		return protocol.Packet{}, errors.New("nil node")
	}
	if p.ID == "" {
		p.ID = protocol.NewID()
	}
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	respCh := make(chan protocol.Packet, 1)
	n.pendMu.Lock()
	if n.pend == nil {
		n.pend = map[string]chan protocol.Packet{}
	}
	n.pend[p.ID] = respCh
	n.pendMu.Unlock()

	if err := n.writePacket(p); err != nil {
		n.pendMu.Lock()
		delete(n.pend, p.ID)
		n.pendMu.Unlock()
		return protocol.Packet{}, err
	}

	select {
	case resp := <-respCh:
		return resp, nil
	case <-time.After(timeout):
		n.pendMu.Lock()
		delete(n.pend, p.ID)
		n.pendMu.Unlock()
		return protocol.Packet{}, errors.New("timeout waiting for reply")
	}
}

func (n *nodeConn) resolveReply(reply protocol.Packet) bool {
	if reply.ReplyTo == "" {
		return false
	}
	n.pendMu.Lock()
	ch := n.pend[reply.ReplyTo]
	if ch != nil {
		delete(n.pend, reply.ReplyTo)
	}
	n.pendMu.Unlock()
	if ch == nil {
		return false
	}
	ch <- reply
	return true
}

func (n *nodeConn) readLoop() {
	for {
		packet, err := protocol.ReadPacket(n.reader)
		if err != nil {
			log.Printf("master: node %s read error: %v", n.id, err)
			return
		}
		if n.resolveReply(packet) {
			continue
		}
		switch packet.Type {
		case string(protocol.PacketHeartbeatReply):
			heartbeat := protocol.Heartbeat{}
			if err := protocol.DecodePayload(packet, &heartbeat); err != nil {
				log.Printf("master: node %s heartbeat reply invalid: %v", n.id, err)
				continue
			}
			log.Printf("master: node %s heartbeat cpu=%.2f ram=%.2f", n.id, heartbeat.Usage.CPUPercent, heartbeat.Usage.RAMPercent)
		case string(protocol.PacketNodeCommandReply):
			log.Printf("master: node %s command reply received", n.id)
		default:
			log.Printf("master: node %s unexpected packet type=%s", n.id, packet.Type)
		}
	}
}

type nodeRegistry struct {
	mu    sync.RWMutex
	nodes map[string]*nodeConn
}

var nodes = nodeRegistry{nodes: map[string]*nodeConn{}}

func registerNode(id string, conn net.Conn, reader *bufio.Reader, writer *bufio.Writer) *nodeConn {
	if id == "" || conn == nil || reader == nil || writer == nil {
		return nil
	}
	n := &nodeConn{id: id, conn: conn, reader: reader, writer: writer, pend: map[string]chan protocol.Packet{}}
	nodes.mu.Lock()
	nodes.nodes[id] = n
	nodes.mu.Unlock()
	return n
}

func unregisterNode(id string) {
	if id == "" {
		return
	}
	nodes.mu.Lock()
	delete(nodes.nodes, id)
	nodes.mu.Unlock()
}

func sendCommandToNode(id string, payload protocol.Packet) error {
	if id == "" {
		return errors.New("node id is empty")
	}
	nodes.mu.RLock()
	n := nodes.nodes[id]
	nodes.mu.RUnlock()
	if n == nil {
		return errors.New("node not connected")
	}
	return n.writePacket(payload)
}

func sendCommandToNodeWait(id string, payload protocol.Packet, timeout time.Duration) (protocol.Packet, error) {
	if id == "" {
		return protocol.Packet{}, errors.New("node id is empty")
	}
	nodes.mu.RLock()
	n := nodes.nodes[id]
	nodes.mu.RUnlock()
	if n == nil {
		return protocol.Packet{}, errors.New("node not connected")
	}
	return n.sendRequest(payload, timeout)
}

func listConnectedNodeIDs() []string {
	nodes.mu.RLock()
	defer nodes.mu.RUnlock()
	ids := make([]string, 0, len(nodes.nodes))
	for id := range nodes.nodes {
		ids = append(ids, id)
	}
	return ids
}
