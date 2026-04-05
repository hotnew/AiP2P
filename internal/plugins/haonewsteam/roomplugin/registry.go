package roomplugin

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"sync"

	teamcore "hao.news/internal/haonews/team"
)

type RoomPlugin interface {
	ID() string
	Manifest() Manifest
	Handler(store *teamcore.Store, teamID string) http.Handler
}

type Manifest struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Version       string   `json:"version"`
	Description   string   `json:"description,omitempty"`
	MessageKinds  []string `json:"messageKinds,omitempty"`
	ArtifactKinds []string `json:"artifactKinds,omitempty"`
}

func LoadManifestJSON(data []byte) (Manifest, error) {
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return Manifest{}, fmt.Errorf("roomplugin: invalid manifest json: %w", err)
	}
	if m.ID == "" {
		return Manifest{}, fmt.Errorf("roomplugin: manifest missing id")
	}
	return m, nil
}

func LoadManifestFile(path string) (Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, err
	}
	return LoadManifestJSON(data)
}

type Registry struct {
	mu      sync.RWMutex
	plugins map[string]RoomPlugin
}

func NewRegistry() *Registry {
	return &Registry{plugins: make(map[string]RoomPlugin)}
}

func (r *Registry) Register(p RoomPlugin) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	id := p.ID()
	if id == "" {
		return fmt.Errorf("roomplugin: empty plugin id")
	}
	if _, exists := r.plugins[id]; exists {
		return fmt.Errorf("roomplugin: duplicate plugin id %q", id)
	}
	r.plugins[id] = p
	return nil
}

func (r *Registry) MustRegister(p RoomPlugin) {
	if err := r.Register(p); err != nil {
		panic(err)
	}
}

func (r *Registry) Get(id string) (RoomPlugin, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.plugins[id]
	return p, ok
}

func (r *Registry) All() []RoomPlugin {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ids := make([]string, 0, len(r.plugins))
	for id := range r.plugins {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	out := make([]RoomPlugin, 0, len(ids))
	for _, id := range ids {
		out = append(out, r.plugins[id])
	}
	return out
}

func (r *Registry) IDs() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.plugins))
	for id := range r.plugins {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

func (r *Registry) Manifests() []Manifest {
	plugins := r.All()
	out := make([]Manifest, 0, len(plugins))
	for _, p := range plugins {
		out = append(out, p.Manifest())
	}
	return out
}
