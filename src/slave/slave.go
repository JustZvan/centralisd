package slave

import (
	"bufio"
	"centralisd/src/core/config"
	"centralisd/src/core/protocol"
	"centralisd/src/slave/firewall"
	"centralisd/src/slave/handlers"
	"centralisd/src/slave/libvirt"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"log"
	"net"
	"os"
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

	if err := protocol.WriteHello(writer, protocol.HeaderMasterSlave); err != nil {
		log.Printf("slave: write hello: %v", err)
		os.Exit(1)
	}
	log.Printf("slave: sent hello")

	clientID, err := protocol.IDFromPublicKey(pubKey)
	if err != nil {
		log.Printf("slave: id from pubkey: %v", err)
		os.Exit(1)
	}
	pubKeyStr := base64.RawURLEncoding.EncodeToString(pubKey)
	authHello := protocol.AuthHello{ID: clientID, PubKey: pubKeyStr, Role: "slave"}
	packet, err := protocol.NewPacket(string(protocol.PacketAuthHello), authHello)
	if err != nil {
		log.Printf("slave: auth packet: %v", err)
		os.Exit(1)
	}
	if err := protocol.WritePacket(writer, packet); err != nil {
		log.Printf("slave: write auth: %v", err)
		os.Exit(1)
	}
	log.Printf("slave: sent auth hello")

	challengePacket, err := protocol.ReadPacket(reader)
	if err != nil {
		log.Printf("slave: read challenge: %v", err)
		os.Exit(1)
	}
	if challengePacket.Type == string(protocol.PacketError) {
		log.Printf("slave: server rejected handshake")
		os.Exit(1)
	}
	if challengePacket.Type != string(protocol.PacketAuthChallenge) {
		log.Printf("slave: unexpected challenge type: %s", challengePacket.Type)
		os.Exit(1)
	}
	challengePayload := protocol.AuthChallenge{}
	if err := protocol.DecodePayload(challengePacket, &challengePayload); err != nil {
		log.Printf("slave: challenge payload: %v", err)
		os.Exit(1)
	}
	log.Printf("slave: challenge received")

	sigStr, err := protocol.SignChallenge(privKey, challengePayload.Challenge)
	if err != nil {
		log.Printf("slave: sign challenge: %v", err)
		os.Exit(1)
	}
	proofPacket, err := protocol.NewPacket(string(protocol.PacketAuthProof), protocol.AuthProof{Signature: sigStr})
	if err != nil {
		log.Printf("slave: proof packet: %v", err)
		os.Exit(1)
	}
	if err := protocol.WritePacket(writer, proofPacket); err != nil {
		log.Printf("slave: write signature: %v", err)
		os.Exit(1)
	}
	log.Printf("slave: sent signature")

	respPacket, err := protocol.ReadPacket(reader)
	if err != nil {
		log.Printf("slave: auth response: %v", err)
		os.Exit(1)
	}
	if respPacket.Type != string(protocol.PacketAuthOK) {
		log.Printf("slave: auth failed type=%s", respPacket.Type)
		return
	}

	log.Printf("slave: connected")
	log.Printf("slave: setting up firewall")

	err = firewall.SetupFirewall(cfg)

	if err != nil {
		log.Printf("slave: firewall failed, this node is insecure!")
		println(err.Error())
	}

	qemu, err := libvirt.GetQEMU()
	networks, err := qemu.ListNetworks()

	for i, element := range networks {
		println(i)
		println(element)
	}

	for {
		packet, err := protocol.ReadPacket(reader)
		if err != nil {
			log.Printf("slave: read packet: %v", err)
			return
		}
		handler, ok := handlers.PacketHandlers[packet.Type]
		if !ok {
			log.Printf("slave: unexpected packet type: %s", packet.Type)
			continue
		}

		if err := handler(writer, packet); err != nil {
			log.Printf("slave: %s handler: %v", packet.Type, err)
			return
		}
	}
}
