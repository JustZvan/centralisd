package registry

import (
    "centralisd/src/core/protocol"
    "crypto/ed25519"
    "encoding/base64"
)

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
    if !protocol.VerifyIDForPublicKey(master.ID, pubKeyBytes) {
        return false
    }
    return protocol.VerifyChallengeSignature(ed25519.PublicKey(pubKeyBytes), challenge, sigB64)
}
