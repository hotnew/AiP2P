package live

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"hao.news/internal/haonews"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
)

func archiveNoticeMetadata(info RoomInfo, result ArchiveResult) map[string]any {
	return map[string]any{
		"title":                strings.TrimSpace(info.Title),
		"creator":              strings.TrimSpace(info.Creator),
		"created_at":           strings.TrimSpace(info.CreatedAt),
		"network_id":           strings.TrimSpace(info.NetworkID),
		"channel":              strings.TrimSpace(info.Channel),
		"description":          strings.TrimSpace(info.Description),
		"archive.channel":      strings.TrimSpace(result.Channel),
		"archive.infohash":     strings.TrimSpace(result.Published.InfoHash),
		"archive.ref":          strings.TrimSpace(result.Published.Ref),
		"archive.content_dir":  strings.TrimSpace(result.Published.ContentDir),
		"archive.viewer_url":   strings.TrimSpace(result.ViewerURL),
		"archive.events":       result.Events,
		"archive.archived_at":  strings.TrimSpace(result.ArchivedAt),
	}
}

func archiveResultFromNotice(event LiveMessage) (ArchiveResult, bool) {
	infoHash := metadataStringValue(event.Payload.Metadata, "archive.infohash")
	if infoHash == "" {
		return ArchiveResult{}, false
	}
	result := ArchiveResult{
		RoomID:     strings.TrimSpace(event.RoomID),
		Channel:    firstNonEmpty(metadataStringValue(event.Payload.Metadata, "archive.channel"), metadataStringValue(event.Payload.Metadata, "channel")),
		Events:     metadataIntValue(event.Payload.Metadata, "archive.events"),
		ArchivedAt: firstNonEmpty(metadataStringValue(event.Payload.Metadata, "archive.archived_at"), strings.TrimSpace(event.Timestamp)),
		ViewerURL:  metadataStringValue(event.Payload.Metadata, "archive.viewer_url"),
		Published: haonews.PublishResult{
			InfoHash:    infoHash,
			Ref:         firstNonEmpty(metadataStringValue(event.Payload.Metadata, "archive.ref"), metadataStringValue(event.Payload.Metadata, "archive.magnet")),
			Magnet:      metadataStringValue(event.Payload.Metadata, "archive.magnet"),
			ContentDir:  metadataStringValue(event.Payload.Metadata, "archive.content_dir"),
		},
	}
	if strings.TrimSpace(result.ViewerURL) == "" {
		result.ViewerURL = "/posts/" + infoHash
	}
	return result, true
}

func archiveSyncRefFromNotice(event LiveMessage) string {
	ref := metadataStringValue(event.Payload.Metadata, "archive.ref")
	if ref != "" {
		return ref
	}
	magnet := metadataStringValue(event.Payload.Metadata, "archive.magnet")
	if magnet != "" {
		return magnet
	}
	return metadataStringValue(event.Payload.Metadata, "archive.infohash")
}

func publishArchiveNoticeDetached(netPath string, identity haonews.AgentIdentity, info RoomInfo, result ArchiveResult) error {
	netPath = strings.TrimSpace(netPath)
	if netPath == "" {
		return fmt.Errorf("net path is required for detached archive notice")
	}
	if err := haonews.EnsureDefaultNetworkBootstrapConfig(netPath); err != nil {
		return err
	}
	netCfg, err := haonews.LoadNetworkBootstrapConfig(netPath)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	host, dhtRuntime, mdnsService, _, ps, err := startTransport(ctx, netCfg)
	if err != nil {
		return err
	}
	defer func() {
		if mdnsService != nil {
			_ = mdnsService.Close()
		}
		if dhtRuntime != nil {
			_ = dhtRuntime.Close()
		}
		if host != nil {
			_ = host.Close()
		}
	}()
	topic, err := ps.Join(RoomAnnounceTopic())
	if err != nil {
		return fmt.Errorf("join archive notice topic: %w", err)
	}
	defer func() { _ = topic.Close() }()
	sub, err := topic.Subscribe()
	if err != nil {
		return fmt.Errorf("subscribe archive notice topic: %w", err)
	}
	defer sub.Cancel()
	msg, err := NewSignedMessage(identity, identity.Author, info.RoomID, TypeArchiveNotice, 1, 0, LivePayload{
		Content:     firstNonEmpty(strings.TrimSpace(result.ViewerURL), "/posts/"+strings.TrimSpace(result.Published.InfoHash)),
		ContentType: "application/json",
		Metadata:    archiveNoticeMetadata(info, result),
	})
	if err != nil {
		return err
	}
	if err := waitForPublishPeers(ctx, topic, 5*time.Second); err != nil {
		return err
	}
	body, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	publishCtx, publishCancel := context.WithTimeout(ctx, 5*time.Second)
	defer publishCancel()
	if err := topic.Publish(publishCtx, body); err != nil {
		return err
	}
	select {
	case <-ctx.Done():
	case <-time.After(1500 * time.Millisecond):
	}
	return nil
}

func waitForPublishPeers(ctx context.Context, topic *pubsub.Topic, waitFor time.Duration) error {
	if topic == nil {
		return fmt.Errorf("topic is required")
	}
	if len(topic.ListPeers()) > 0 {
		return nil
	}
	waitCtx, cancel := context.WithTimeout(ctx, waitFor)
	defer cancel()
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-waitCtx.Done():
			if len(topic.ListPeers()) > 0 {
				return nil
			}
			return fmt.Errorf("no live announcement peers available")
		case <-ticker.C:
			if len(topic.ListPeers()) > 0 {
				return nil
			}
		}
	}
}

func metadataIntValue(metadata map[string]any, key string) int {
	if len(metadata) == 0 {
		return 0
	}
	value, ok := metadata[key]
	if !ok || value == nil {
		return 0
	}
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case float32:
		return int(typed)
	default:
		var out int
		if _, err := fmt.Sscanf(strings.TrimSpace(fmt.Sprint(value)), "%d", &out); err == nil {
			return out
		}
		return 0
	}
}
