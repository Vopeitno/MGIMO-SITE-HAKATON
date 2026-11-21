package password

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"golang.org/x/crypto/argon2"
	"strings"
)

var workSlots = make(chan struct{}, 2)

func Hash(value string) (string, error) {
	workSlots <- struct{}{}
	defer func() { <-workSlots }()
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(value), salt, 3, 64*1024, 2, 32)
	return fmt.Sprintf("$argon2id$v=19$m=65536,t=3,p=2$%s$%s", base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(key)), nil
}

func Verify(value, encoded string) bool {
	p := strings.Split(encoded, "$")
	if len(p) != 6 || p[1] != "argon2id" || p[2] != "v=19" {
		return false
	}
	var memory, time uint32
	var threads uint8
	if _, err := fmt.Sscanf(p[3], "m=%d,t=%d,p=%d", &memory, &time, &threads); err != nil {
		return false
	}
	if memory < 8192 || memory > 128*1024 || time < 1 || time > 6 || threads < 1 || threads > 4 {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(p[4])
	if err != nil || len(salt) < 8 || len(salt) > 64 {
		return false
	}
	key, err := base64.RawStdEncoding.DecodeString(p[5])
	if err != nil || len(key) != 32 {
		return false
	}
	workSlots <- struct{}{}
	defer func() { <-workSlots }()
	actual := argon2.IDKey([]byte(value), salt, time, memory, threads, uint32(len(key)))
	return subtle.ConstantTimeCompare(key, actual) == 1
}
