package utils

import (
	"hash/fnv"
	"math/rand"
	"strconv"
	"strings"
)

// Charset for random string generation
const Charset = "abcdefghijklmnopqrstuvwxyz0123456789"

// StringWithCharset generates a random string with the specified length and charset
func StringWithCharset(length int, charset string) string {
	randombytes := make([]byte, length)
	for i := range randombytes {
		num := rand.Intn(len(charset))
		randombytes[i] = charset[num]
	}

	return string(randombytes)
}

// SanitizeInput sanitizes input strings
func SanitizeInput(input string) string {
	input = strings.ReplaceAll(input, "\n", "")
	input = strings.ReplaceAll(input, "\r", "")
	return input
}

// HashGroupKey creates a Kubernetes-compatible hash from a groupKey
// Uses FNV-1a hash which is fast and has good distribution
func HashGroupKey(groupKey string) string {
	if groupKey == "" {
		return ""
	}

	// Use FNV-1a hash for fast, non-cryptographic hashing
	h := fnv.New64a()
	h.Write([]byte(groupKey))
	hash := h.Sum64()

	// Convert to string and ensure it starts with a letter (Kubernetes requirement)
	hashStr := "g" + strconv.FormatUint(hash, 36)

	// Ensure it's within Kubernetes label length limit (63 characters)
	if len(hashStr) > 63 {
		hashStr = hashStr[:63]
	}

	return hashStr
}
