package master

import (
	"bufio"
	"centralisd/src/core/protocol"
	"crypto/ed25519"
	"encoding/base64"
	"log"
	"net"
	"os"
	"strconv"
	"time"
)

func handleConnection(conn net.Conn, allowedNodes map[string]struct{}) {
	defer conn.Close()
	remote := conn.RemoteAddr().String()
	log.Printf("master: incoming connection from %s", remote)

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	if err := protocol.ReadHello(reader, protocol.HeaderMasterSlave); err != nil {
		log.Printf("master: %s bad hello %v", remote, err)
		_ = protocol.WritePacket(writer, protocol.NewError("invalid hello"))
		return
	}

	authPacket, err := protocol.ReadPacket(reader)
	if err != nil {
		return
	}
	if authPacket.Type != string(protocol.PacketAuthHello) {
		log.Printf("master: %s bad auth type %q", remote, authPacket.Type)
		_ = protocol.WritePacket(writer, protocol.NewError("auth wrong type"))
		return
	}
	authHello := protocol.AuthHello{}
	if err := protocol.DecodePayload(authPacket, &authHello); err != nil {
		log.Printf("master: %s invalid auth payload: %v", remote, err)
		_ = protocol.WritePacket(writer, protocol.NewError("auth payload invalid"))
		return
	}
	if authHello.Role != "slave" {
		log.Printf("master: %s invalid role %q", remote, authHello.Role)
		_ = protocol.WritePacket(writer, protocol.NewError("auth wrong role"))
		return
	}
	pubKeyBytes, err := base64.RawURLEncoding.DecodeString(authHello.PubKey)
	if err != nil {
		log.Printf("master: %s invalid pubkey b64: %v", remote, err)
		_ = protocol.WritePacket(writer, protocol.NewError("invalid pubkey"))
		return
	}
	if !protocol.VerifyIDForPublicKey(authHello.ID, pubKeyBytes) {
		log.Printf("master: %s client id mismatch id=%s", remote, authHello.ID)
		_ = protocol.WritePacket(writer, protocol.NewError("id mismatch"))
		return
	}
	if len(allowedNodes) > 0 {
		if _, ok := allowedNodes[authHello.ID]; !ok {
			log.Printf("master: %s rejected: node not whitelisted id=%s", remote, authHello.ID)
			_ = protocol.WritePacket(writer, protocol.NewError("node not allowed"))
			return
		}
	}

	challenge := protocol.GenerateChallenge()
	challengePacket, _ := protocol.NewPacket(string(protocol.PacketAuthChallenge), protocol.AuthChallenge{Challenge: challenge})
	if err := protocol.WritePacket(writer, challengePacket); err != nil {
		return
	}

	proofPacket, err := protocol.ReadPacket(reader)
	if err != nil {
		return
	}
	if proofPacket.Type != string(protocol.PacketAuthProof) {
		log.Printf("master: %s invalid proof type %q", remote, proofPacket.Type)
		_ = protocol.WritePacket(writer, protocol.NewError("auth proof wrong type"))
		return
	}
	proof := protocol.AuthProof{}
	if err := protocol.DecodePayload(proofPacket, &proof); err != nil {
		log.Printf("master: %s invalid proof payload: %v", remote, err)
		_ = protocol.WritePacket(writer, protocol.NewError("auth proof invalid"))
		return
	}
	pubKey := ed25519.PublicKey(pubKeyBytes)
	if !protocol.VerifyChallengeSignature(pubKey, challenge, proof.Signature) {
		log.Printf("master: %s auth failed id=%s", remote, authHello.ID)
		_ = protocol.WritePacket(writer, protocol.NewError("auth failed"))
		return
	}
	log.Printf("master: %s auth ok id=%s", remote, authHello.ID)
	okPacket, _ := protocol.NewPacket(string(protocol.PacketAuthOK), nil)
	_ = protocol.WritePacket(writer, okPacket)

	node := registerNode(authHello.ID, conn, reader, writer)
	go node.readLoop()
	defer unregisterNode(authHello.ID)

	for {
		packet, err := protocol.NewPacket(string(protocol.PacketHeartbeat), nil)
		if err != nil {
			return
		}
		if _, err := node.sendRequest(packet, 5*time.Second); err != nil {
			log.Printf("master: %s heartbeat request: %v", remote, err)
			return
		}
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
