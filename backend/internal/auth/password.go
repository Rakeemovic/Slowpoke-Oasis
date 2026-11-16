package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Argon2id parameters, following the OWASP minimum recommendation.
// They are stored with each hash, so they can be raised later without
// breaking existing passwords.
const (
	argonTime    uint32 = 2
	argonMemory  uint32 = 19 * 1024 // KiB
	argonThreads uint8  = 1
	argonKeyLen  uint32 = 32
	argonSaltLen        = 16
)

// hashPassword returns an encoded Argon2id hash with a random salt.
// Format: argon2id$<memory>$<time>$<threads>$<salt>$<key>
func hashPassword(password string) string {
	salt := make([]byte, argonSaltLen)
	rand.Read(salt) // never fails: crypto/rand aborts the program instead
	key := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	return fmt.Sprintf("argon2id$%d$%d$%d$%s$%s",
		argonMemory, argonTime, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	)
}

// verifyPassword reports whether password matches an encoded hash.
// The comparison runs in constant time.
func verifyPassword(password, encoded string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[0] != "argon2id" {
		return false
	}
	memory, err1 := strconv.ParseUint(parts[1], 10, 32)
	time, err2 := strconv.ParseUint(parts[2], 10, 32)
	threads, err3 := strconv.ParseUint(parts[3], 10, 8)
	salt, err4 := base64.RawStdEncoding.DecodeString(parts[4])
	want, err5 := base64.RawStdEncoding.DecodeString(parts[5])
	if err1 != nil || err2 != nil || err3 != nil || err4 != nil || err5 != nil {
		return false
	}

	got := argon2.IDKey([]byte(password), salt, uint32(time), uint32(memory), uint8(threads), uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1
}

// dummyHash is checked during logins for unknown usernames, so that the
// response takes about as long as for a real account.
var dummyHash = hashPassword("timing-equalizer")
