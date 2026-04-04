package team

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"
)

const (
	webhookDeliveryStatusPending    = "pending"
	webhookDeliveryStatusRetrying   = "retrying"
	webhookDeliveryStatusDelivered  = "delivered"
	webhookDeliveryStatusFailed     = "failed"
	webhookDeliveryStatusDeadLetter = "dead_letter"
)

type WebhookDeliveryRecord struct {
	DeliveryID    string    `json:"delivery_id"`
	TeamID        string    `json:"team_id"`
	WebhookID     string    `json:"webhook_id,omitempty"`
	URL           string    `json:"url"`
	Token         string    `json:"token,omitempty"`
	Events        []string  `json:"events,omitempty"`
	Event         TeamEvent `json:"event"`
	Status        string    `json:"status"`
	Attempt       int       `json:"attempt"`
	StatusCode    int       `json:"status_code,omitempty"`
	Error         string    `json:"error,omitempty"`
	NextRetryAt   time.Time `json:"next_retry_at,omitempty"`
	LastAttemptAt time.Time `json:"last_attempt_at,omitempty"`
	DeliveredAt   time.Time `json:"delivered_at,omitempty"`
	CreatedAt     time.Time `json:"created_at,omitempty"`
	UpdatedAt     time.Time `json:"updated_at,omitempty"`
	ReplayedFrom  string    `json:"replayed_from,omitempty"`
}

type WebhookDeliveryStatus struct {
	TeamID          string                  `json:"team_id"`
	RetryingCount   int                     `json:"retrying_count"`
	DeliveredCount  int                     `json:"delivered_count"`
	FailedCount     int                     `json:"failed_count"`
	DeadLetterCount int                     `json:"dead_letter_count"`
	RecentFailures  []WebhookDeliveryRecord `json:"recent_failures,omitempty"`
	RecentDead      []WebhookDeliveryRecord `json:"recent_dead_letters,omitempty"`
	RecentDelivered []WebhookDeliveryRecord `json:"recent_delivered,omitempty"`
}

func (s *Store) LoadWebhookDeliveryStatusCtx(ctx context.Context, teamID string) (WebhookDeliveryStatus, error) {
	if err := ctxErr(ctx); err != nil {
		return WebhookDeliveryStatus{}, err
	}
	teamID = NormalizeTeamID(teamID)
	if teamID == "" {
		return WebhookDeliveryStatus{}, errors.New("empty team id")
	}
	records, err := s.loadWebhookDeliveriesNoCtx(teamID)
	if err != nil {
		return WebhookDeliveryStatus{}, err
	}
	sortWebhookDeliveries(records)
	status := WebhookDeliveryStatus{TeamID: teamID}
	for _, record := range records {
		switch record.Status {
		case webhookDeliveryStatusRetrying:
			status.RetryingCount++
		case webhookDeliveryStatusDelivered:
			status.DeliveredCount++
			if len(status.RecentDelivered) < 10 {
				status.RecentDelivered = append(status.RecentDelivered, record)
			}
		case webhookDeliveryStatusFailed:
			status.FailedCount++
			if len(status.RecentFailures) < 10 {
				status.RecentFailures = append(status.RecentFailures, record)
			}
		case webhookDeliveryStatusDeadLetter:
			status.DeadLetterCount++
			if len(status.RecentDead) < 10 {
				status.RecentDead = append(status.RecentDead, record)
			}
		}
	}
	return status, nil
}

func (s *Store) ReplayWebhookDeliveryCtx(ctx context.Context, teamID, deliveryID string) (WebhookDeliveryRecord, error) {
	if err := ctxErr(ctx); err != nil {
		return WebhookDeliveryRecord{}, err
	}
	teamID = NormalizeTeamID(teamID)
	deliveryID = strings.TrimSpace(deliveryID)
	if teamID == "" {
		return WebhookDeliveryRecord{}, errors.New("empty team id")
	}
	if deliveryID == "" {
		return WebhookDeliveryRecord{}, errors.New("empty delivery id")
	}
	record, err := s.loadWebhookDeliveryNoCtx(teamID, deliveryID)
	if err != nil {
		return WebhookDeliveryRecord{}, err
	}
	cfg := PushNotificationConfig{
		WebhookID: record.WebhookID,
		URL:       record.URL,
		Token:     record.Token,
		Events:    append([]string(nil), record.Events...),
		UpdatedAt: time.Now().UTC(),
	}
	replayID := buildWebhookDeliveryID(record.Event, cfg.WebhookID, time.Now().UTC())
	s.sendWebhookWithRecord(cfg, record.Event, replayID, deliveryID)
	return s.loadWebhookDeliveryNoCtx(teamID, replayID)
}

func (s *Store) loadWebhookDeliveriesNoCtx(teamID string) ([]WebhookDeliveryRecord, error) {
	if s == nil {
		return nil, errors.New("nil team store")
	}
	teamID = NormalizeTeamID(teamID)
	if teamID == "" {
		return nil, errors.New("empty team id")
	}
	data, err := os.ReadFile(s.webhookDeliveryPath(teamID))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var records []WebhookDeliveryRecord
	if err := json.Unmarshal(data, &records); err != nil {
		return nil, err
	}
	for i := range records {
		records[i] = normalizeWebhookDeliveryRecord(records[i])
	}
	return records, nil
}

func (s *Store) loadWebhookDeliveryNoCtx(teamID, deliveryID string) (WebhookDeliveryRecord, error) {
	records, err := s.loadWebhookDeliveriesNoCtx(teamID)
	if err != nil {
		return WebhookDeliveryRecord{}, err
	}
	for _, record := range records {
		if record.DeliveryID == deliveryID {
			return record, nil
		}
	}
	return WebhookDeliveryRecord{}, os.ErrNotExist
}

func (s *Store) saveWebhookDeliveriesLocked(teamID string, records []WebhookDeliveryRecord) error {
	path := s.webhookDeliveryPath(teamID)
	if err := os.MkdirAll(filepathDir(path), 0o755); err != nil {
		return err
	}
	body, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return err
	}
	body = append(body, '\n')
	return os.WriteFile(path, body, 0o644)
}

func (s *Store) updateWebhookDeliveryLocked(teamID string, record WebhookDeliveryRecord) error {
	records, err := s.loadWebhookDeliveriesNoCtx(teamID)
	if err != nil {
		return err
	}
	record = normalizeWebhookDeliveryRecord(record)
	replaced := false
	for i := range records {
		if records[i].DeliveryID != record.DeliveryID {
			continue
		}
		records[i] = record
		replaced = true
		break
	}
	if !replaced {
		records = append(records, record)
	}
	sortWebhookDeliveries(records)
	if len(records) > 200 {
		records = records[:200]
	}
	return s.saveWebhookDeliveriesLocked(teamID, records)
}

func sortWebhookDeliveries(records []WebhookDeliveryRecord) {
	sort.SliceStable(records, func(i, j int) bool {
		if !records[i].UpdatedAt.Equal(records[j].UpdatedAt) {
			return records[i].UpdatedAt.After(records[j].UpdatedAt)
		}
		return records[i].DeliveryID > records[j].DeliveryID
	})
}

func normalizeWebhookDeliveryRecord(record WebhookDeliveryRecord) WebhookDeliveryRecord {
	record.DeliveryID = strings.TrimSpace(record.DeliveryID)
	record.TeamID = NormalizeTeamID(record.TeamID)
	record.WebhookID = strings.TrimSpace(record.WebhookID)
	record.URL = strings.TrimSpace(record.URL)
	record.Token = strings.TrimSpace(record.Token)
	record.Events = normalizeNonEmptyStrings(record.Events)
	record.Status = strings.TrimSpace(record.Status)
	if record.CreatedAt.IsZero() {
		record.CreatedAt = time.Now().UTC()
	}
	if record.UpdatedAt.IsZero() {
		record.UpdatedAt = record.CreatedAt
	}
	return record
}

func buildWebhookDeliveryID(event TeamEvent, webhookID string, at time.Time) string {
	webhookID = strings.TrimSpace(webhookID)
	if webhookID == "" {
		webhookID = "webhook"
	}
	return sanitizeArchiveID(webhookID + "-" + event.EventID + "-" + at.UTC().Format("20060102T150405.000000000Z"))
}

func isWebhookRetriableStatus(code int) bool {
	return code == http.StatusTooManyRequests || code >= 500
}

func webhookNextRetryAt(at time.Time, attempt int) time.Time {
	if attempt < 1 {
		attempt = 1
	}
	return at.Add(time.Duration(attempt) * 200 * time.Millisecond)
}

func filepathDir(path string) string {
	idx := strings.LastIndex(path, string(os.PathSeparator))
	if idx < 0 {
		return "."
	}
	return path[:idx]
}
