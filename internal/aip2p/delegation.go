package aip2p

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const LatestWriterDelegationVersion = "aip2p-delegation/0.1"

type DelegationKind string

const (
	DelegationKindWriterDelegation DelegationKind = "writer_delegation"
)

type WriterDelegation struct {
	Type            DelegationKind `json:"type"`
	Version         string         `json:"version"`
	ParentAgentID   string         `json:"parent_agent_id"`
	ParentKeyType   string         `json:"parent_key_type"`
	ParentPublicKey string         `json:"parent_public_key"`
	ChildAgentID    string         `json:"child_agent_id"`
	ChildKeyType    string         `json:"child_key_type"`
	ChildPublicKey  string         `json:"child_public_key"`
	Scopes          []string       `json:"scopes,omitempty"`
	CreatedAt       string         `json:"created_at"`
	ExpiresAt       string         `json:"expires_at,omitempty"`
	Signature       string         `json:"signature"`
}

type unsignedWriterDelegation struct {
	Type            DelegationKind `json:"type"`
	Version         string         `json:"version"`
	ParentAgentID   string         `json:"parent_agent_id"`
	ParentKeyType   string         `json:"parent_key_type"`
	ParentPublicKey string         `json:"parent_public_key"`
	ChildAgentID    string         `json:"child_agent_id"`
	ChildKeyType    string         `json:"child_key_type"`
	ChildPublicKey  string         `json:"child_public_key"`
	Scopes          []string       `json:"scopes,omitempty"`
	CreatedAt       string         `json:"created_at"`
	ExpiresAt       string         `json:"expires_at,omitempty"`
}

func (d *WriterDelegation) Normalize() {
	if d == nil {
		return
	}
	d.Type = DelegationKind(strings.TrimSpace(string(d.Type)))
	if d.Type == "" {
		d.Type = DelegationKindWriterDelegation
	}
	d.Version = strings.TrimSpace(d.Version)
	if d.Version == "" {
		d.Version = LatestWriterDelegationVersion
	}
	d.ParentAgentID = strings.TrimSpace(d.ParentAgentID)
	d.ParentKeyType = strings.TrimSpace(d.ParentKeyType)
	if d.ParentKeyType == "" {
		d.ParentKeyType = KeyTypeEd25519
	}
	d.ParentPublicKey = strings.ToLower(strings.TrimSpace(d.ParentPublicKey))
	d.ChildAgentID = strings.TrimSpace(d.ChildAgentID)
	d.ChildKeyType = strings.TrimSpace(d.ChildKeyType)
	if d.ChildKeyType == "" {
		d.ChildKeyType = KeyTypeEd25519
	}
	d.ChildPublicKey = strings.ToLower(strings.TrimSpace(d.ChildPublicKey))
	d.Scopes = uniqueDelegationScopes(d.Scopes)
	d.CreatedAt = strings.TrimSpace(d.CreatedAt)
	d.ExpiresAt = strings.TrimSpace(d.ExpiresAt)
	d.Signature = strings.ToLower(strings.TrimSpace(d.Signature))
}

func uniqueDelegationScopes(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(items))
	out := make([]string, 0, len(items))
	for _, item := range items {
		key := strings.ToLower(strings.TrimSpace(item))
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, key)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func (d WriterDelegation) payloadBytes() ([]byte, error) {
	d.Normalize()
	return json.Marshal(unsignedWriterDelegation{
		Type:            d.Type,
		Version:         d.Version,
		ParentAgentID:   d.ParentAgentID,
		ParentKeyType:   d.ParentKeyType,
		ParentPublicKey: d.ParentPublicKey,
		ChildAgentID:    d.ChildAgentID,
		ChildKeyType:    d.ChildKeyType,
		ChildPublicKey:  d.ChildPublicKey,
		Scopes:          d.Scopes,
		CreatedAt:       d.CreatedAt,
		ExpiresAt:       d.ExpiresAt,
	})
}

func ValidateWriterDelegation(delegation WriterDelegation) error {
	delegation.Normalize()
	if delegation.Type != DelegationKindWriterDelegation {
		return fmt.Errorf("unsupported delegation type %q", delegation.Type)
	}
	if delegation.Version != LatestWriterDelegationVersion {
		return fmt.Errorf("unsupported delegation version %q", delegation.Version)
	}
	if delegation.ParentPublicKey == "" || delegation.ChildPublicKey == "" {
		return errors.New("parent_public_key and child_public_key are required")
	}
	if delegation.ParentKeyType != KeyTypeEd25519 || delegation.ChildKeyType != KeyTypeEd25519 {
		return errors.New("only ed25519 delegations are supported")
	}
	if _, err := time.Parse(time.RFC3339, delegation.CreatedAt); err != nil {
		return errors.New("created_at must be RFC3339")
	}
	if delegation.ExpiresAt != "" {
		if _, err := time.Parse(time.RFC3339, delegation.ExpiresAt); err != nil {
			return errors.New("expires_at must be RFC3339")
		}
	}
	parentPublicKey, err := decodeHexKey(delegation.ParentPublicKey, ed25519.PublicKeySize, "parent_public_key")
	if err != nil {
		return err
	}
	if _, err := decodeHexKey(delegation.ChildPublicKey, ed25519.PublicKeySize, "child_public_key"); err != nil {
		return err
	}
	signature, err := decodeHexKey(delegation.Signature, ed25519.SignatureSize, "signature")
	if err != nil {
		return err
	}
	payload, err := delegation.payloadBytes()
	if err != nil {
		return err
	}
	if !ed25519.Verify(ed25519.PublicKey(parentPublicKey), payload, signature) {
		return errors.New("delegation signature verification failed")
	}
	return nil
}

func SignWriterDelegation(delegation WriterDelegation, identity AgentIdentity) (WriterDelegation, error) {
	privateKey, err := signingPrivateKey(identity)
	if err != nil {
		return WriterDelegation{}, err
	}
	delegation.Normalize()
	payload, err := delegation.payloadBytes()
	if err != nil {
		return WriterDelegation{}, err
	}
	delegation.Signature = hex.EncodeToString(ed25519.Sign(privateKey, payload))
	if err := ValidateWriterDelegation(delegation); err != nil {
		return WriterDelegation{}, err
	}
	return delegation, nil
}

func BuildChildWriterDelegation(parent, child AgentIdentity, createdAt time.Time) (WriterDelegation, error) {
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	delegation := WriterDelegation{
		Type:            DelegationKindWriterDelegation,
		Version:         LatestWriterDelegationVersion,
		ParentAgentID:   strings.TrimSpace(parent.AgentID),
		ParentKeyType:   KeyTypeEd25519,
		ParentPublicKey: strings.ToLower(strings.TrimSpace(parent.PublicKey)),
		ChildAgentID:    strings.TrimSpace(child.AgentID),
		ChildKeyType:    KeyTypeEd25519,
		ChildPublicKey:  strings.ToLower(strings.TrimSpace(child.PublicKey)),
		Scopes:          []string{"publish:any"},
		CreatedAt:       createdAt.UTC().Format(time.RFC3339),
	}
	return SignWriterDelegation(delegation, parent)
}

func LoadWriterDelegation(path string) (WriterDelegation, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return WriterDelegation{}, err
	}
	return ParseWriterDelegationJSON(data)
}

func ParseWriterDelegationJSON(data []byte) (WriterDelegation, error) {
	var delegation WriterDelegation
	if err := json.Unmarshal(data, &delegation); err != nil {
		return WriterDelegation{}, err
	}
	if err := ValidateWriterDelegation(delegation); err != nil {
		return WriterDelegation{}, err
	}
	return delegation, nil
}

func WriterDelegationFromAny(value any) (*WriterDelegation, bool, error) {
	if value == nil {
		return nil, false, nil
	}
	switch typed := value.(type) {
	case WriterDelegation:
		typed.Normalize()
		return &typed, true, nil
	case *WriterDelegation:
		if typed == nil {
			return nil, false, nil
		}
		copyValue := *typed
		copyValue.Normalize()
		return &copyValue, true, nil
	case map[string]any:
		data, err := json.Marshal(typed)
		if err != nil {
			return nil, true, err
		}
		item, err := ParseWriterDelegationJSON(data)
		if err != nil {
			return nil, true, err
		}
		return &item, true, nil
	case json.RawMessage:
		item, err := ParseWriterDelegationJSON(typed)
		if err != nil {
			return nil, true, err
		}
		return &item, true, nil
	case string:
		item, err := ParseWriterDelegationJSON([]byte(typed))
		if err != nil {
			return nil, true, err
		}
		return &item, true, nil
	default:
		return nil, true, fmt.Errorf("unsupported hd.delegation payload type %T", value)
	}
}

func WriterDelegationToMap(delegation WriterDelegation) (map[string]any, error) {
	delegation.Normalize()
	data, err := json.Marshal(delegation)
	if err != nil {
		return nil, err
	}
	var value map[string]any
	if err := json.Unmarshal(data, &value); err != nil {
		return nil, err
	}
	return value, nil
}

func LoadDelegationProofForChild(identityFile string, identity AgentIdentity) (*WriterDelegation, bool, error) {
	identityFile = strings.TrimSpace(identityFile)
	if identityFile == "" {
		return nil, false, nil
	}
	rootDir := filepath.Dir(filepath.Dir(identityFile))
	delegationDir := filepath.Join(rootDir, "delegations")
	entries, err := os.ReadDir(delegationDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	now := time.Now().UTC()
	candidates := make([]WriterDelegation, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		item, err := LoadWriterDelegation(filepath.Join(delegationDir, entry.Name()))
		if err != nil {
			continue
		}
		if strings.ToLower(strings.TrimSpace(item.ChildPublicKey)) != strings.ToLower(strings.TrimSpace(identity.PublicKey)) {
			continue
		}
		if strings.ToLower(strings.TrimSpace(item.ParentPublicKey)) != strings.ToLower(strings.TrimSpace(identity.ParentPublicKey)) {
			continue
		}
		if item.ExpiresAt != "" {
			expiresAt, err := time.Parse(time.RFC3339, item.ExpiresAt)
			if err != nil || !expiresAt.After(now) {
				continue
			}
		}
		candidates = append(candidates, item)
	}
	if len(candidates) == 0 {
		return nil, false, nil
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].CreatedAt > candidates[j].CreatedAt
	})
	selected := candidates[0]
	return &selected, true, nil
}
