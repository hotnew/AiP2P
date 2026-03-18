package aip2p

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var slugUnsafe = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

type Store struct {
	Root       string
	DataDir    string
	TorrentDir string
}

func OpenStore(root string) (*Store, error) {
	if strings.TrimSpace(root) == "" {
		root = ".aip2p"
	}
	store := &Store{
		Root:       root,
		DataDir:    filepath.Join(root, "data"),
		TorrentDir: filepath.Join(root, "torrents"),
	}
	for _, dir := range []string{store.Root, store.DataDir, store.TorrentDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}
	return store, nil
}

func (s *Store) NewContentDir(title string, now time.Time) string {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	prefix := now.UTC().Format("20060102T150405Z")
	name := "message"
	if clean := slugify(title); clean != "" {
		name = clean
	}
	return filepath.Join(s.DataDir, fmt.Sprintf("%s-%s", prefix, name))
}

func (s *Store) TorrentPath(infoHash string) string {
	return filepath.Join(s.TorrentDir, strings.ToLower(infoHash)+".torrent")
}

func slugify(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	value = slugUnsafe.ReplaceAllString(value, "-")
	value = strings.Trim(value, "-.")
	if len(value) > 48 {
		value = value[:48]
	}
	return strings.ToLower(value)
}
