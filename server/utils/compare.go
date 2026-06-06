package utils

import "crypto/subtle"

func ConstantTimeEqualString(a string, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
