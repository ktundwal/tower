package core

import (
	"crypto/rand"
	"encoding/base32"
	"fmt"
	"time"

	"tower/internal/contracts"
)

var crockfordEncoding = base32.NewEncoding("0123456789ABCDEFGHJKMNPQRSTVWXYZ").WithPadding(base32.NoPadding)

func NewSessionID(now time.Time) contracts.SessionID {
	return contracts.SessionID(newULID(now))
}

func NewRuntimeID(now time.Time) contracts.RuntimeID {
	return contracts.RuntimeID(newULID(now))
}

func NewEventID(now time.Time) contracts.EventID {
	return contracts.EventID(newULID(now))
}

func newULID(now time.Time) string {
	var raw [16]byte
	timestamp := uint64(now.UTC().UnixMilli())

	raw[0] = byte(timestamp >> 40)
	raw[1] = byte(timestamp >> 32)
	raw[2] = byte(timestamp >> 24)
	raw[3] = byte(timestamp >> 16)
	raw[4] = byte(timestamp >> 8)
	raw[5] = byte(timestamp)

	if _, err := rand.Read(raw[6:]); err != nil {
		panic(fmt.Errorf("generate bootstrap ulid entropy: %w", err))
	}

	return crockfordEncoding.EncodeToString(raw[:])
}
