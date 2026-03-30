package haonews

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const KeyTypeEd25519 = "ed25519"

type AgentIdentity struct {
	AgentID         string `json:"agent_id"`
	Author          string `json:"author,omitempty"`
	KeyType         string `json:"key_type"`
	PublicKey       string `json:"public_key"`
	PrivateKey      string `json:"private_key,omitempty"`
	CreatedAt       string `json:"created_at"`
	HDEnabled       bool   `json:"hd_enabled,omitempty"`
	Mnemonic        string `json:"mnemonic,omitempty"`
	MasterPubKey    string `json:"master_pubkey,omitempty"`
	DerivationPath  string `json:"derivation_path,omitempty"`
	Parent          string `json:"parent,omitempty"`
	ParentPublicKey string `json:"parent_public_key,omitempty"`
	WriterDelegation *WriterDelegation `json:"writer_delegation,omitempty"`
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

func NewHDMasterIdentity(agentID, author, mnemonic string, createdAt time.Time) (AgentIdentity, error) {
	agentID = strings.TrimSpace(agentID)
	author = strings.TrimSpace(author)
	if agentID == "" {
		return AgentIdentity{}, errors.New("agent_id is required")
	}
	if author == "" {
		return AgentIdentity{}, errors.New("author is required")
	}
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	mnemonic = strings.TrimSpace(mnemonic)
	if mnemonic == "" {
		var err error
		mnemonic, err = GenerateMnemonic()
		if err != nil {
			return AgentIdentity{}, err
		}
	}
	seed, err := MnemonicToSeed(mnemonic)
	if err != nil {
		return AgentIdentity{}, err
	}
	path, err := PathFromURI(author)
	if err != nil {
		return AgentIdentity{}, err
	}
	publicKey, _, _, err := DeriveHDKey(seed, path)
	if err != nil {
		return AgentIdentity{}, err
	}
	return AgentIdentity{
		AgentID:        agentID,
		Author:         author,
		KeyType:        KeyTypeEd25519,
		PublicKey:      publicKey,
		CreatedAt:      createdAt.UTC().Format(time.RFC3339),
		HDEnabled:      true,
		Mnemonic:       mnemonic,
		MasterPubKey:   publicKey,
		DerivationPath: path,
	}, nil
}

func RecoverHDIdentity(agentID, author, mnemonic string, createdAt time.Time) (AgentIdentity, error) {
	return NewHDMasterIdentity(agentID, author, mnemonic, createdAt)
}

func DeriveChildIdentity(identity AgentIdentity, author string, createdAt time.Time) (AgentIdentity, error) {
	if err := identity.Validate(); err != nil {
		return AgentIdentity{}, err
	}
	if !identity.HDEnabled || strings.TrimSpace(identity.Mnemonic) == "" {
		return AgentIdentity{}, errors.New("identity does not contain HD mnemonic material")
	}
	rootAuthor, err := RootAuthor(author)
	if err != nil {
		return AgentIdentity{}, err
	}
	if strings.TrimSpace(identity.Author) != rootAuthor {
		return AgentIdentity{}, errors.New("child author does not belong to the supplied master identity")
	}
	path, err := PathFromURI(author)
	if err != nil {
		return AgentIdentity{}, err
	}
	seed, err := MnemonicToSeed(identity.Mnemonic)
	if err != nil {
		return AgentIdentity{}, err
	}
	publicKey, privateKey, _, err := DeriveHDKey(seed, path)
	if err != nil {
		return AgentIdentity{}, err
	}
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	childIdentity := AgentIdentity{
		AgentID:         identity.AgentID,
		Author:          strings.TrimSpace(author),
		KeyType:         KeyTypeEd25519,
		PublicKey:       publicKey,
		PrivateKey:      privateKey,
		CreatedAt:       createdAt.UTC().Format(time.RFC3339),
		HDEnabled:       true,
		MasterPubKey:    identity.MasterPubKey,
		DerivationPath:  path,
		Parent:          identity.Author,
		ParentPublicKey: identity.PublicKey,
	}
	delegation, err := BuildChildWriterDelegation(identity, childIdentity, createdAt)
	if err != nil {
		return AgentIdentity{}, err
	}
	childIdentity.WriterDelegation = &delegation
	return childIdentity, nil
}

func SaveAgentIdentity(path string, identity AgentIdentity) error {
	if err := identity.Validate(); err != nil {
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
	identity, err = enrichIdentityDelegation(path, identity)
	if err != nil {
		return AgentIdentity{}, err
	}
	if err := identity.Validate(); err != nil {
		return AgentIdentity{}, err
	}
	return identity, nil
}

func enrichIdentityDelegation(path string, identity AgentIdentity) (AgentIdentity, error) {
	if identity.WriterDelegation != nil || !requiresDelegationForIdentity(identity) {
		return identity, nil
	}
	if delegation, ok, err := LoadDelegationProofForChild(path, identity); err != nil {
		return AgentIdentity{}, err
	} else if ok {
		identity.WriterDelegation = delegation
		return identity, nil
	}
	rootIdentity, ok, err := findLocalHDParentIdentity(path, identity)
	if err != nil {
		return AgentIdentity{}, err
	}
	if !ok {
		return identity, nil
	}
	createdAt, _ := time.Parse(time.RFC3339, strings.TrimSpace(identity.CreatedAt))
	delegation, err := BuildChildWriterDelegation(rootIdentity, identity, createdAt)
	if err != nil {
		return AgentIdentity{}, err
	}
	identity.WriterDelegation = &delegation
	return identity, nil
}

func findLocalHDParentIdentity(path string, child AgentIdentity) (AgentIdentity, bool, error) {
	identityDir := filepath.Dir(path)
	entries, err := os.ReadDir(identityDir)
	if err != nil {
		return AgentIdentity{}, false, err
	}
	selfPath, _ := filepath.Abs(path)
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		candidatePath := filepath.Join(identityDir, entry.Name())
		absCandidate, _ := filepath.Abs(candidatePath)
		if absCandidate == selfPath {
			continue
		}
		data, err := os.ReadFile(candidatePath)
		if err != nil {
			continue
		}
		var candidate AgentIdentity
		if err := json.Unmarshal(data, &candidate); err != nil {
			continue
		}
		if err := candidate.Validate(); err != nil {
			continue
		}
		if !candidate.HDEnabled || strings.TrimSpace(candidate.Mnemonic) == "" {
			continue
		}
		if strings.TrimSpace(candidate.Author) != strings.TrimSpace(child.Parent) {
			continue
		}
		if strings.ToLower(strings.TrimSpace(candidate.PublicKey)) != strings.ToLower(strings.TrimSpace(child.ParentPublicKey)) {
			continue
		}
		return candidate, true, nil
	}
	return AgentIdentity{}, false, nil
}

func (id AgentIdentity) Validate() error {
	id.AgentID = strings.TrimSpace(id.AgentID)
	id.Author = strings.TrimSpace(id.Author)
	id.KeyType = strings.TrimSpace(id.KeyType)
	id.PublicKey = strings.ToLower(strings.TrimSpace(id.PublicKey))
	id.PrivateKey = strings.ToLower(strings.TrimSpace(id.PrivateKey))
	id.MasterPubKey = strings.ToLower(strings.TrimSpace(id.MasterPubKey))
	id.DerivationPath = strings.TrimSpace(id.DerivationPath)
	id.Parent = strings.TrimSpace(id.Parent)
	id.ParentPublicKey = strings.ToLower(strings.TrimSpace(id.ParentPublicKey))
	id.Mnemonic = strings.TrimSpace(id.Mnemonic)
	if id.AgentID == "" {
		return errors.New("agent_id is required")
	}
	if id.KeyType != KeyTypeEd25519 {
		return fmt.Errorf("unsupported key_type %q", id.KeyType)
	}
	if _, err := time.Parse(time.RFC3339, strings.TrimSpace(id.CreatedAt)); err != nil {
		return errors.New("created_at must be RFC3339")
	}
	if id.HDEnabled {
		return id.validateHD()
	}
	if id.PrivateKey == "" {
		return errors.New("private_key is required")
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

func (id AgentIdentity) ValidatePrivate() error {
	if err := id.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(id.PrivateKey) == "" && strings.TrimSpace(id.Mnemonic) == "" {
		return errors.New("identity does not contain signing material")
	}
	return nil
}

func BuildSignedOrigin(msg Message, identity AgentIdentity) (*MessageOrigin, map[string]any, error) {
	signingIdentity, extensions, err := resolveSigningIdentity(identity, msg.Author, msg.Extensions)
	if err != nil {
		return nil, nil, err
	}
	origin := MessageOrigin{
		Author:    strings.TrimSpace(msg.Author),
		AgentID:   strings.TrimSpace(signingIdentity.AgentID),
		KeyType:   KeyTypeEd25519,
		PublicKey: strings.ToLower(strings.TrimSpace(signingIdentity.PublicKey)),
	}
	msg.Extensions = cloneMap(extensions)
	payload, err := signedMessagePayloadBytes(msg, origin)
	if err != nil {
		return nil, nil, err
	}
	privateKeyBytes, err := decodeHexKey(signingIdentity.PrivateKey, ed25519.PrivateKeySize, "private_key")
	if err != nil {
		return nil, nil, err
	}
	origin.Signature = hex.EncodeToString(ed25519.Sign(ed25519.PrivateKey(privateKeyBytes), payload))
	return &origin, extensions, nil
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
	if err := ValidateSignedMetadata(strings.TrimSpace(msg.Author), origin.PublicKey, msg.Extensions); err != nil {
		return err
	}
	return nil
}

func ValidateSignedMetadata(author, originPublicKey string, extensions map[string]any) error {
	msg := Message{
		Author:     strings.TrimSpace(author),
		Extensions: cloneMap(extensions),
	}
	if err := validateSignedKeyMetadata(msg, originPublicKey); err != nil {
		return err
	}
	if err := validateHDOriginMetadata(msg); err != nil {
		return err
	}
	if err := validateHDDelegationMetadata(msg, originPublicKey); err != nil {
		return err
	}
	return nil
}

func (id AgentIdentity) validateHD() error {
	if id.Author == "" {
		return errors.New("author is required for hd identities")
	}
	if id.DerivationPath == "" {
		return errors.New("derivation_path is required for hd identities")
	}
	if _, err := ParseDerivationPath(id.DerivationPath); err != nil {
		return err
	}
	if id.Mnemonic != "" {
		seed, err := MnemonicToSeed(id.Mnemonic)
		if err != nil {
			return err
		}
		publicKey, privateKey, _, err := DeriveHDKey(seed, id.DerivationPath)
		if err != nil {
			return err
		}
		if id.PublicKey != "" && id.PublicKey != publicKey {
			return errors.New("public_key does not match derived mnemonic key")
		}
		if id.PrivateKey != "" && id.PrivateKey != privateKey {
			return errors.New("private_key does not match derived mnemonic key")
		}
		if id.MasterPubKey != "" && id.Parent == "" && id.MasterPubKey != publicKey {
			return errors.New("master_pubkey does not match hd root key")
		}
		return nil
	}
	if id.PublicKey == "" {
		return errors.New("public_key is required")
	}
	if _, err := decodeHexKey(id.PublicKey, ed25519.PublicKeySize, "public_key"); err != nil {
		return err
	}
	if id.PrivateKey != "" {
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
	}
	if id.Parent != "" && id.ParentPublicKey != "" {
		if _, err := decodeHexKey(id.ParentPublicKey, ed25519.PublicKeySize, "parent_public_key"); err != nil {
			return err
		}
	}
	if id.WriterDelegation != nil {
		delegation := *id.WriterDelegation
		if err := ValidateWriterDelegation(delegation); err != nil {
			return fmt.Errorf("writer_delegation: %w", err)
		}
		if strings.ToLower(strings.TrimSpace(delegation.ChildPublicKey)) != strings.ToLower(strings.TrimSpace(id.PublicKey)) {
			return errors.New("writer_delegation.child_public_key must match identity public_key")
		}
		if strings.TrimSpace(id.ParentPublicKey) != "" &&
			strings.ToLower(strings.TrimSpace(delegation.ParentPublicKey)) != strings.ToLower(strings.TrimSpace(id.ParentPublicKey)) {
			return errors.New("writer_delegation.parent_public_key must match identity parent_public_key")
		}
	}
	return nil
}

func resolveSigningIdentity(identity AgentIdentity, author string, extensions map[string]any) (AgentIdentity, map[string]any, error) {
	if err := identity.ValidatePrivate(); err != nil {
		return AgentIdentity{}, nil, err
	}
	author = strings.TrimSpace(author)
	if !identity.HDEnabled {
		if identity.Author != "" && identity.Author != author {
			return AgentIdentity{}, nil, errors.New("author does not match identity-file author")
		}
		result := cloneMap(extensions)
		applySignedKeyMetadata(result, identity.PublicKey, identity.PublicKey)
		if err := applyDelegationMetadata(result, identity); err != nil {
			return AgentIdentity{}, nil, err
		}
		return identity, result, nil
	}
	if strings.TrimSpace(identity.Mnemonic) != "" {
		return deriveSigningIdentity(identity, author, extensions)
	}
	if identity.Author != "" && identity.Author != author {
		return AgentIdentity{}, nil, errors.New("author does not match identity-file author")
	}
	result := cloneMap(extensions)
	if identity.Parent != "" {
		result["hd.parent"] = identity.Parent
		if identity.ParentPublicKey != "" {
			result["hd.parent_pubkey"] = identity.ParentPublicKey
		}
		result["hd.path"] = identity.DerivationPath
	}
	parentPublicKey := identity.ParentPublicKey
	if strings.TrimSpace(parentPublicKey) == "" {
		parentPublicKey = identity.PublicKey
	}
	applySignedKeyMetadata(result, identity.PublicKey, parentPublicKey)
	if err := applyDelegationMetadata(result, identity); err != nil {
		return AgentIdentity{}, nil, err
	}
	return identity, result, nil
}

func ResolveSigningIdentity(identity AgentIdentity, author string, extensions map[string]any) (AgentIdentity, map[string]any, error) {
	return resolveSigningIdentity(identity, author, extensions)
}

func deriveSigningIdentity(identity AgentIdentity, author string, extensions map[string]any) (AgentIdentity, map[string]any, error) {
	rootAuthor, err := RootAuthor(author)
	if err != nil {
		return AgentIdentity{}, nil, err
	}
	if strings.TrimSpace(identity.Author) != rootAuthor {
		return AgentIdentity{}, nil, errors.New("author does not belong to hd identity root")
	}
	path, err := PathFromURI(author)
	if err != nil {
		return AgentIdentity{}, nil, err
	}
	seed, err := MnemonicToSeed(identity.Mnemonic)
	if err != nil {
		return AgentIdentity{}, nil, err
	}
	publicKey, privateKey, _, err := DeriveHDKey(seed, path)
	if err != nil {
		return AgentIdentity{}, nil, err
	}
	result := cloneMap(extensions)
	if author != identity.Author {
		result["hd.parent"] = identity.Author
		result["hd.parent_pubkey"] = identity.PublicKey
		result["hd.path"] = path
	}
	applySignedKeyMetadata(result, publicKey, identity.PublicKey)
	signingIdentity := AgentIdentity{
		AgentID:         identity.AgentID,
		Author:          author,
		KeyType:         KeyTypeEd25519,
		PublicKey:       publicKey,
		PrivateKey:      privateKey,
		CreatedAt:       identity.CreatedAt,
		HDEnabled:       true,
		MasterPubKey:    identity.PublicKey,
		DerivationPath:  path,
		Parent:          identity.Author,
		ParentPublicKey: identity.PublicKey,
	}
		if author != identity.Author {
			createdAt, _ := time.Parse(time.RFC3339, strings.TrimSpace(identity.CreatedAt))
			delegation, err := BuildChildWriterDelegation(identity, signingIdentity, createdAt)
			if err != nil {
				return AgentIdentity{}, nil, err
			}
		signingIdentity.WriterDelegation = &delegation
	}
	if err := applyDelegationMetadata(result, signingIdentity); err != nil {
		return AgentIdentity{}, nil, err
	}
	return signingIdentity, result, nil
}

func applySignedKeyMetadata(extensions map[string]any, originPublicKey, parentPublicKey string) {
	if extensions == nil {
		return
	}
	if originPublicKey = strings.ToLower(strings.TrimSpace(originPublicKey)); originPublicKey != "" {
		extensions["origin_public_key"] = originPublicKey
	}
	if parentPublicKey = strings.ToLower(strings.TrimSpace(parentPublicKey)); parentPublicKey != "" {
		extensions["parent_public_key"] = parentPublicKey
	}
}

func applyDelegationMetadata(extensions map[string]any, identity AgentIdentity) error {
	if extensions == nil {
		return nil
	}
	if !requiresDelegationForIdentity(identity) {
		delete(extensions, "hd.delegation")
		return nil
	}
	if identity.WriterDelegation == nil {
		return errors.New("child signing identities must include writer_delegation")
	}
	delegation := *identity.WriterDelegation
	if err := ValidateWriterDelegation(delegation); err != nil {
		return fmt.Errorf("writer_delegation: %w", err)
	}
		value, err := WriterDelegationToMap(delegation)
		if err != nil {
			return fmt.Errorf("writer_delegation: %w", err)
		}
		extensions["hd.delegation"] = value
		return nil
	}

func requiresDelegationForIdentity(identity AgentIdentity) bool {
	return strings.TrimSpace(identity.Parent) != "" &&
		strings.TrimSpace(identity.ParentPublicKey) != "" &&
		strings.ToLower(strings.TrimSpace(identity.ParentPublicKey)) != strings.ToLower(strings.TrimSpace(identity.PublicKey))
}

func validateSignedKeyMetadata(msg Message, originPublicKey string) error {
	metadataOrigin, hasOriginMetadata := stringFromMap(msg.Extensions, "origin_public_key")
	metadataParent, hasParentMetadata := stringFromMap(msg.Extensions, "parent_public_key")
	if !hasOriginMetadata || !hasParentMetadata {
		return validateLegacySignedKeyMetadata(msg, originPublicKey, hasOriginMetadata, hasParentMetadata)
	}
	metadataOrigin = strings.ToLower(strings.TrimSpace(metadataOrigin))
	metadataParent = strings.ToLower(strings.TrimSpace(metadataParent))
	originPublicKey = strings.ToLower(strings.TrimSpace(originPublicKey))
	if metadataOrigin != originPublicKey {
		return errors.New("origin_public_key must match origin.public_key")
	}
	if _, err := decodeHexKey(metadataOrigin, ed25519.PublicKeySize, "origin_public_key"); err != nil {
		return err
	}
	if _, err := decodeHexKey(metadataParent, ed25519.PublicKeySize, "parent_public_key"); err != nil {
		return err
	}
	return nil
}

func validateLegacySignedKeyMetadata(msg Message, originPublicKey string, hasOriginMetadata, hasParentMetadata bool) error {
	if hasOriginMetadata != hasParentMetadata {
		if !hasOriginMetadata {
			return errors.New("signed messages must include origin_public_key")
		}
		return errors.New("signed messages must include parent_public_key")
	}
	if hasOriginMetadata && hasParentMetadata {
		return nil
	}
	if _, err := decodeHexKey(originPublicKey, ed25519.PublicKeySize, "origin.public_key"); err != nil {
		return err
	}
	if parentPubKey, ok := stringFromMap(msg.Extensions, "hd.parent_pubkey"); ok {
		if _, err := decodeHexKey(parentPubKey, ed25519.PublicKeySize, "hd.parent_pubkey"); err != nil {
			return err
		}
	}
	return nil
}

func validateHDOriginMetadata(msg Message) error {
	parent, okParent := stringFromMap(msg.Extensions, "hd.parent")
	parentPubKey, okParentPubKey := stringFromMap(msg.Extensions, "hd.parent_pubkey")
	path, okPath := stringFromMap(msg.Extensions, "hd.path")
	count := 0
	for _, ok := range []bool{okParent, okParentPubKey, okPath} {
		if ok {
			count++
		}
	}
	if count == 0 {
		return nil
	}
	if count != 3 {
		return errors.New("hd origin metadata must include hd.parent, hd.parent_pubkey, and hd.path together")
	}
	rootAuthor, err := RootAuthor(msg.Author)
	if err != nil {
		return err
	}
	if rootAuthor == strings.TrimSpace(msg.Author) {
		return errors.New("hd origin metadata is only valid for child authors")
	}
	if strings.TrimSpace(parent) != rootAuthor {
		return errors.New("hd.parent must match the root author for child identities")
	}
	if _, err := decodeHexKey(parentPubKey, ed25519.PublicKeySize, "hd.parent_pubkey"); err != nil {
		return err
	}
	if _, err := ParseDerivationPath(path); err != nil {
		return err
	}
	expectedPath, err := PathFromURI(msg.Author)
	if err != nil {
		return err
	}
	if strings.TrimSpace(path) != expectedPath {
		return errors.New("hd.path must match the child author derivation path")
	}
	return nil
}

func validateHDDelegationMetadata(msg Message, originPublicKey string) error {
	if !signedMessageRequiresDelegation(msg, originPublicKey) {
		return nil
	}
	parentPublicKey, ok := stringFromMap(msg.Extensions, "parent_public_key")
	if !ok {
		return errors.New("child signed messages must include parent_public_key")
	}
	value, ok := msg.Extensions["hd.delegation"]
	if !ok {
		return errors.New("child signed messages must include hd.delegation")
	}
	delegation, present, err := WriterDelegationFromAny(value)
	if err != nil {
		return err
	}
	if !present || delegation == nil {
		return errors.New("child signed messages must include hd.delegation")
	}
	if err := ValidateWriterDelegation(*delegation); err != nil {
		return err
	}
	if strings.ToLower(strings.TrimSpace(delegation.ChildPublicKey)) != strings.ToLower(strings.TrimSpace(originPublicKey)) {
		return errors.New("hd.delegation.child_public_key must match origin_public_key")
	}
	if strings.ToLower(strings.TrimSpace(delegation.ParentPublicKey)) != strings.ToLower(strings.TrimSpace(parentPublicKey)) {
		return errors.New("hd.delegation.parent_public_key must match parent_public_key")
	}
	if parentPubKey, ok := stringFromMap(msg.Extensions, "hd.parent_pubkey"); ok {
		if strings.ToLower(strings.TrimSpace(parentPubKey)) != strings.ToLower(strings.TrimSpace(delegation.ParentPublicKey)) {
			return errors.New("hd.delegation.parent_public_key must match hd.parent_pubkey")
		}
	}
	return nil
}

func signedMessageRequiresDelegation(msg Message, originPublicKey string) bool {
	parentPublicKey, hasParent := stringFromMap(msg.Extensions, "parent_public_key")
	if hasParent && strings.ToLower(strings.TrimSpace(parentPublicKey)) != strings.ToLower(strings.TrimSpace(originPublicKey)) {
		return true
	}
	_, okParent := stringFromMap(msg.Extensions, "hd.parent")
	_, okParentPubKey := stringFromMap(msg.Extensions, "hd.parent_pubkey")
	_, okPath := stringFromMap(msg.Extensions, "hd.path")
	return okParent || okParentPubKey || okPath
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

func stringFromMap(values map[string]any, key string) (string, bool) {
	if len(values) == 0 {
		return "", false
	}
	value, ok := values[key]
	if !ok {
		return "", false
	}
	text, ok := value.(string)
	if !ok {
		return "", false
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return "", false
	}
	return text, true
}
