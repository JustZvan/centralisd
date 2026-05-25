package tcp

import (
	"bufio"
	"centralisd/src/orchestrator/registry"
	"encoding/json"
	"errors"
	"log"
	"net"
	"strings"
	"time"
)

// Simple line-based protocol (v1):
// client -> server: CENTRALISD-ORCH/1\n
// client -> server: JSON line messages, one per line.
// Supported message types:
// {"type":"master.auth","master":{...},"signature":"..."}
// {"type":"master.register","master":{...}}
// {"type":"master.heartbeat","master":{...}}
// server -> client: OK\n or FAIL\n

type envelope struct {
	Type   string              `json:"type"`
	Master *registry.MasterInfo `json:"master,omitempty"`
	// Signature is base64url(raw(ed25519.Sign(privKey, challenge)))
	Signature string `json:"signature,omitempty"`
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

	first, err := reader.ReadString('\n')
	if err != nil {
		return
	}
	first = strings.TrimSpace(first)
	if first != "CENTRALISD-ORCH/1" {
		log.Printf("orchestrator: %s bad hello %q", remote, first)
		_, _ = writer.WriteString("FAIL\n")
		_ = writer.Flush()
		return
	}

	// After hello, allow long-lived connections.
	_ = conn.SetDeadline(time.Time{})

	// Challenge-response: client must prove it owns the master key.
	challenge := registry.GenerateChallenge()
	_, _ = writer.WriteString(challenge + "\n")
	_ = writer.Flush()

	challengeRespLine, err := reader.ReadString('\n')
	if err != nil {
		log.Printf("orchestrator: %s read auth response: %v", remote, err)
		return
	}
	challengeRespLine = strings.TrimSpace(challengeRespLine)
	resp := envelope{}
	if err := json.Unmarshal([]byte(challengeRespLine), &resp); err != nil {
		log.Printf("orchestrator: %s invalid auth json: %v", remote, err)
		_, _ = writer.WriteString("FAIL\n")
		_ = writer.Flush()
		return
	}
	if resp.Type != "master.auth" || resp.Master == nil {
		log.Printf("orchestrator: %s auth wrong type=%q", remote, resp.Type)
		_, _ = writer.WriteString("FAIL\n")
		_ = writer.Flush()
		return
	}
	if !registry.VerifyMasterAuth(challenge, *resp.Master, resp.Signature) {
		log.Printf("orchestrator: %s auth verify failed master_id=%s cluster=%s", remote, resp.Master.ID, resp.Master.Cluster)
		_, _ = writer.WriteString("FAIL\n")
		_ = writer.Flush()
		return
	}
	if len(masterWhitelist) > 0 {
		clusterSet, ok := masterWhitelist[resp.Master.Cluster]
		if !ok {
			log.Printf("orchestrator: %s auth rejected: cluster not whitelisted cluster=%s master_id=%s", remote, resp.Master.Cluster, resp.Master.ID)
			_, _ = writer.WriteString("FAIL\n")
			_ = writer.Flush()
			return
		}
		if _, ok := clusterSet[resp.Master.ID]; !ok {
			log.Printf("orchestrator: %s auth rejected: master id not whitelisted cluster=%s master_id=%s", remote, resp.Master.Cluster, resp.Master.ID)
			_, _ = writer.WriteString("FAIL\n")
			_ = writer.Flush()
			return
		}
	}
	_, _ = writer.WriteString("OK\n")
	_ = writer.Flush()
	log.Printf("orchestrator: %s auth ok cluster=%s master_id=%s", remote, resp.Master.Cluster, resp.Master.ID)

	// Pin the authenticated master ID for this connection.
	authedMasterID := resp.Master.ID
	authedClusterID := resp.Master.Cluster

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			log.Printf("orchestrator: %s read error: %v", remote, err)
			return
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		env := envelope{}
		if err := json.Unmarshal([]byte(line), &env); err != nil {
			log.Printf("orchestrator: %s invalid json: %v", remote, err)
			_, _ = writer.WriteString("FAIL\n")
			_ = writer.Flush()
			continue
		}
		if env.Master == nil || env.Master.ID == "" {
			_, _ = writer.WriteString("FAIL\n")
			_ = writer.Flush()
			continue
		}
		if env.Master.ID != authedMasterID {
			log.Printf("orchestrator: %s rejected: master id changed authed=%s got=%s", remote, authedMasterID, env.Master.ID)
			_, _ = writer.WriteString("FAIL\n")
			_ = writer.Flush()
			continue
		}
		if env.Master.Cluster != authedClusterID {
			log.Printf("orchestrator: %s rejected: cluster changed authed=%s got=%s", remote, authedClusterID, env.Master.Cluster)
			_, _ = writer.WriteString("FAIL\n")
			_ = writer.Flush()
			continue
		}
		switch env.Type {
		case "master.register", "master.heartbeat":
			if store.UpsertMaster(*env.Master) {
				_, _ = writer.WriteString("OK\n")
				_ = writer.Flush()
				if env.Type == "master.register" {
					log.Printf("orchestrator: %s registered master_id=%s cluster=%s", remote, env.Master.ID, env.Master.Cluster)
				}
			} else {
				log.Printf("orchestrator: %s rejected update type=%s master_id=%s cluster=%s", remote, env.Type, env.Master.ID, env.Master.Cluster)
				_, _ = writer.WriteString("FAIL\n")
				_ = writer.Flush()
			}
		default:
			log.Printf("orchestrator: %s unknown message type=%q", remote, env.Type)
			_, _ = writer.WriteString("FAIL\n")
			_ = writer.Flush()
		}
	}
}
