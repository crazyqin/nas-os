package passkey

import "crypto/sha256"

func init() {
	sha256HashImpl = func(s string) []byte {
		h := sha256.Sum256([]byte(s))
		return h[:]
	}
}

// sha256HashImpl is overridden in init()
var sha256HashImpl = func(s string) []byte {
	return nil
}
