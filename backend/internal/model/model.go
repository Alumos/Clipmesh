package model

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

// Clip is the common representation returned to web clients.
// Binary content is kept on disk; Formats contains textual clipboard formats
// such as text/plain and text/html.
type Clip struct {
	ID         string            `json:"id"`
	UserID     string            `json:"-"`
	Kind       string            `json:"kind"`
	DeviceID   string            `json:"deviceId"`
	DeviceName string            `json:"deviceName"`
	MimeType   string            `json:"mimeType,omitempty"`
	Name       string            `json:"name,omitempty"`
	Size       int64             `json:"size,omitempty"`
	Formats    map[string]string `json:"formats,omitempty"`
	Preview    string            `json:"preview,omitempty"`
	CreatedAt  time.Time         `json:"createdAt"`
	ExpiresAt  *time.Time        `json:"expiresAt,omitempty"`

	// StoragePath is an implementation detail and is never serialized.
	StoragePath string `json:"-"`
}

// User is the public account representation. Password hashes are deliberately
// kept out of this type so handlers cannot accidentally serialize them.
type User struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"createdAt"`
}

func (u User) IsAdmin() bool {
	return u.Role == "admin"
}

func NewID() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw[:]), nil
}
