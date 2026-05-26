package protocol

import (
    "bufio"
    "crypto/ed25519"
    "crypto/rand"
    "crypto/sha256"
    "encoding/base64"
    "errors"
    "fmt"
    "strings"
)

const (
    HeaderMasterSlave  = "CENTRALISD-MS/1"
    HeaderOrchestrator = "CENTRALISD-ORCH/1"
)

func IDFromPublicKey(pubKey ed25519.PublicKey) (string, error) {
    if len(pubKey) != ed25519.PublicKeySize {
        return "", fmt.Errorf("invalid ed25519 public key length")
    }
    sum := sha256.Sum256(pubKey)
    return base64.RawURLEncoding.EncodeToString(sum[:]), nil
}

func VerifyIDForPublicKey(id string, pubKey []byte) bool {
    id = strings.TrimSpace(id)
    if id == "" {
        return false
    }
    sum := sha256.Sum256(pubKey)
    expected := base64.RawURLEncoding.EncodeToString(sum[:])
    return id == expected
}

func GenerateChallenge() string {
    b := make([]byte, 64)
    _, _ = rand.Read(b)
    return base64.RawURLEncoding.EncodeToString(b)
}

func SignChallenge(privKey ed25519.PrivateKey, challenge string) (string, error) {
    if len(privKey) == 0 {
        return "", errors.New("private key missing")
    }
    challenge = strings.TrimSpace(challenge)
    if challenge == "" {
        return "", errors.New("challenge empty")
    }
    sig := ed25519.Sign(privKey, []byte(challenge))
    return base64.RawURLEncoding.EncodeToString(sig), nil
}

func VerifyChallengeSignature(pubKey ed25519.PublicKey, challenge, sigB64 string) bool {
    if len(pubKey) != ed25519.PublicKeySize {
        return false
    }
    sig, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(sigB64))
    if err != nil {
        return false
    }
    return ed25519.Verify(pubKey, []byte(challenge), sig)
}

func WriteHello(writer *bufio.Writer, header string) error {
    header = strings.TrimSpace(header)
    if header == "" {
        return errors.New("header empty")
    }
    return WriteLine(writer, header)
}

func ReadHello(reader *bufio.Reader, expected string) error {
    expected = strings.TrimSpace(expected)
    if expected == "" {
        return errors.New("expected header empty")
    }
    line, err := ReadLine(reader)
    if err != nil {
        return err
    }
    if line != expected {
        return fmt.Errorf("unexpected header %q", line)
    }
    return nil
}
