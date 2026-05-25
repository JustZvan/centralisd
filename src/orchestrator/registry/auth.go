package registry

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
)

func GenerateChallenge() string {
	b := make([]byte, 64)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

// VerifyMasterAuth checks that the presented master ID matches sha256(pubkey)
// and that the signature is a valid ed25519 signature over the challenge.
// master.ID must be base64url(raw(sha256(pubkey))).
func VerifyMasterAuth(challenge string, master MasterInfo, sigB64 string) bool {
	pubKeyBytes, err := base64.RawURLEncoding.DecodeString(master.PubKey)
	if err != nil {
		return false
	}
	if len(pubKeyBytes) != ed25519.PublicKeySize {
		return false
	}
	sum := sha256.Sum256(pubKeyBytes)
	idRaw, err := base64.RawURLEncoding.DecodeString(master.ID)
	if err != nil {
		return false
	}
	if len(idRaw) != len(sum[:]) {
		return false
	}
	for i := range sum {
		if sum[i] != idRaw[i] {
			return false
		}
	}

	sig, err := base64.RawURLEncoding.DecodeString(sigB64)
	if err != nil {
		return false
	}
	return ed25519.Verify(ed25519.PublicKey(pubKeyBytes), []byte(challenge), sig)
}
