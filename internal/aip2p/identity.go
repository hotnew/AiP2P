package aip2p

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

const KeyTypeEd25519 = "ed25519"

type AgentIdentity struct {
	AgentID    string `json:"agent_id"`
	Author     string `json:"author,omitempty"`
	KeyType    string `json:"key_type"`
	PublicKey  string `json:"public_key"`
	PrivateKey string `json:"private_key"`
	CreatedAt  string `json:"created_at"`
}

type signedOriginPayload struct {
	Author    string `json:"author"`
	AgentID   string `json:"agent_id"`
	KeyType   string `json:"key_type"`
	PublicKey string `json:"public_key"`
}

type signedMessagePayload struct {
	Protocol   string              `json:"protocol"`
	Kind       string              `json:"kind"`
	Author     string              `json:"author"`
	CreatedAt  string              `json:"created_at"`
	Channel    string              `json:"channel,omitempty"`
	Title      string              `json:"title,omitempty"`
	BodyFile   string              `json:"body_file"`
	BodySHA256 string              `json:"body_sha256"`
	ReplyTo    *MessageLink        `json:"reply_to,omitempty"`
	Tags       []string            `json:"tags,omitempty"`
	Origin     signedOriginPayload `json:"origin"`
	Extensions map[string]any      `json:"extensions,omitempty"`
}

func NewAgentIdentity(agentID, author string, createdAt time.Time) (AgentIdentity, error) {
	agentID = strings.TrimSpace(agentID)
	author = strings.TrimSpace(author)
	if agentID == "" {
		return AgentIdentity{}, errors.New("agent_id is required")
	}
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return AgentIdentity{}, err
	}
	return AgentIdentity{
		AgentID:    agentID,
		Author:     author,
		KeyType:    KeyTypeEd25519,
		PublicKey:  hex.EncodeToString(publicKey),
		PrivateKey: hex.EncodeToString(privateKey),
		CreatedAt:  createdAt.UTC().Format(time.RFC3339),
	}, nil
}

func SaveAgentIdentity(path string, identity AgentIdentity) error {
	if err := identity.ValidatePrivate(); err != nil {
		return err
	}
	data, err := json.MarshalIndent(identity, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o600)
}

func LoadAgentIdentity(path string) (AgentIdentity, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return AgentIdentity{}, err
	}
	var identity AgentIdentity
	if err := json.Unmarshal(data, &identity); err != nil {
		return AgentIdentity{}, err
	}
	if err := identity.ValidatePrivate(); err != nil {
		return AgentIdentity{}, err
	}
	return identity, nil
}

func (id AgentIdentity) ValidatePrivate() error {
	id.AgentID = strings.TrimSpace(id.AgentID)
	id.Author = strings.TrimSpace(id.Author)
	id.KeyType = strings.TrimSpace(id.KeyType)
	id.PublicKey = strings.ToLower(strings.TrimSpace(id.PublicKey))
	id.PrivateKey = strings.ToLower(strings.TrimSpace(id.PrivateKey))
	if id.AgentID == "" {
		return errors.New("agent_id is required")
	}
	if id.KeyType != KeyTypeEd25519 {
		return fmt.Errorf("unsupported key_type %q", id.KeyType)
	}
	if _, err := time.Parse(time.RFC3339, strings.TrimSpace(id.CreatedAt)); err != nil {
		return errors.New("created_at must be RFC3339")
	}
	publicKey, err := decodeHexKey(id.PublicKey, ed25519.PublicKeySize, "public_key")
	if err != nil {
		return err
	}
	privateKey, err := decodeHexKey(id.PrivateKey, ed25519.PrivateKeySize, "private_key")
	if err != nil {
		return err
	}
	derived := ed25519.PrivateKey(privateKey).Public().(ed25519.PublicKey)
	if !ed25519.PublicKey(publicKey).Equal(derived) {
		return errors.New("private_key does not match public_key")
	}
	return nil
}

func BuildSignedOrigin(msg Message, identity AgentIdentity) (*MessageOrigin, error) {
	if err := identity.ValidatePrivate(); err != nil {
		return nil, err
	}
	if identity.Author != "" && identity.Author != strings.TrimSpace(msg.Author) {
		return nil, errors.New("identity author does not match message author")
	}
	origin := MessageOrigin{
		Author:    strings.TrimSpace(msg.Author),
		AgentID:   strings.TrimSpace(identity.AgentID),
		KeyType:   KeyTypeEd25519,
		PublicKey: strings.ToLower(strings.TrimSpace(identity.PublicKey)),
	}
	payload, err := signedMessagePayloadBytes(msg, origin)
	if err != nil {
		return nil, err
	}
	privateKeyBytes, err := decodeHexKey(identity.PrivateKey, ed25519.PrivateKeySize, "private_key")
	if err != nil {
		return nil, err
	}
	origin.Signature = hex.EncodeToString(ed25519.Sign(ed25519.PrivateKey(privateKeyBytes), payload))
	return &origin, nil
}

func ValidateMessageOrigin(msg Message) error {
	if msg.Origin == nil {
		return nil
	}
	origin := MessageOrigin{
		Author:    strings.TrimSpace(msg.Origin.Author),
		AgentID:   strings.TrimSpace(msg.Origin.AgentID),
		KeyType:   strings.TrimSpace(msg.Origin.KeyType),
		PublicKey: strings.ToLower(strings.TrimSpace(msg.Origin.PublicKey)),
		Signature: strings.ToLower(strings.TrimSpace(msg.Origin.Signature)),
	}
	if origin.Author == "" {
		return errors.New("origin.author is required when origin is present")
	}
	if origin.Author != strings.TrimSpace(msg.Author) {
		return errors.New("origin.author must match author")
	}
	if origin.AgentID == "" {
		return errors.New("origin.agent_id is required when origin is present")
	}
	if origin.KeyType != KeyTypeEd25519 {
		return fmt.Errorf("unsupported origin key_type %q", origin.KeyType)
	}
	publicKey, err := decodeHexKey(origin.PublicKey, ed25519.PublicKeySize, "origin.public_key")
	if err != nil {
		return err
	}
	signature, err := decodeHexKey(origin.Signature, ed25519.SignatureSize, "origin.signature")
	if err != nil {
		return err
	}
	payload, err := signedMessagePayloadBytes(msg, origin)
	if err != nil {
		return err
	}
	if !ed25519.Verify(ed25519.PublicKey(publicKey), payload, signature) {
		return errors.New("origin signature verification failed")
	}
	return nil
}

func signedMessagePayloadBytes(msg Message, origin MessageOrigin) ([]byte, error) {
	payload := signedMessagePayload{
		Protocol:   strings.TrimSpace(msg.Protocol),
		Kind:       strings.TrimSpace(msg.Kind),
		Author:     strings.TrimSpace(msg.Author),
		CreatedAt:  strings.TrimSpace(msg.CreatedAt),
		Channel:    strings.TrimSpace(msg.Channel),
		Title:      strings.TrimSpace(msg.Title),
		BodyFile:   strings.TrimSpace(msg.BodyFile),
		BodySHA256: strings.TrimSpace(msg.BodySHA256),
		ReplyTo:    canonicalMessageLink(msg.ReplyTo),
		Tags:       cleanTags(msg.Tags),
		Origin: signedOriginPayload{
			Author:    strings.TrimSpace(origin.Author),
			AgentID:   strings.TrimSpace(origin.AgentID),
			KeyType:   strings.TrimSpace(origin.KeyType),
			PublicKey: strings.ToLower(strings.TrimSpace(origin.PublicKey)),
		},
		Extensions: cloneMap(msg.Extensions),
	}
	return json.Marshal(payload)
}

func decodeHexKey(raw string, size int, label string) ([]byte, error) {
	value, err := hex.DecodeString(strings.ToLower(strings.TrimSpace(raw)))
	if err != nil {
		return nil, fmt.Errorf("%s must be lowercase hex: %w", label, err)
	}
	if len(value) != size {
		return nil, fmt.Errorf("%s must be %d bytes", label, size)
	}
	return value, nil
}
