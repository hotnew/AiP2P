package team

import (
	"context"
	"errors"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

func ctxErr(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

func (s *Store) ListTeamsCtx(ctx context.Context) ([]Summary, error) {
	if s == nil {
		return nil, nil
	}
	if err := ctxErr(ctx); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(s.root)
	if err != nil {
		return nil, err
	}
	teamIDs := make([]string, 0, len(entries))
	for _, entry := range entries {
		if err := ctxErr(ctx); err != nil {
			return nil, err
		}
		if !entry.IsDir() {
			continue
		}
		teamID := NormalizeTeamID(entry.Name())
		if teamID == "" {
			continue
		}
		teamIDs = append(teamIDs, teamID)
	}
	out := make([]Summary, 0, len(teamIDs))
	type result struct {
		summary Summary
		ok      bool
		err     error
	}
	results := make(chan result, len(teamIDs))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 8)
	for _, teamID := range teamIDs {
		if err := ctxErr(ctx); err != nil {
			return nil, err
		}
		wg.Add(1)
		go func(teamID string) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				results <- result{err: ctx.Err()}
				return
			}
			defer func() { <-sem }()

			info, err := s.LoadTeamCtx(ctx, teamID)
			if err != nil {
				results <- result{err: err}
				return
			}
			members, err := s.LoadMembersCtx(ctx, teamID)
			if err != nil {
				results <- result{err: err}
				return
			}
			channels, err := s.ListChannelsCtx(ctx, teamID)
			channelCount := len(teamChannels(info))
			if err == nil && len(channels) > 0 {
				channelCount = len(channels)
			}
			results <- result{
				summary: Summary{
					Info:         info,
					MemberCount:  len(members),
					ChannelCount: channelCount,
				},
				ok: true,
			}
		}(teamID)
	}
	wg.Wait()
	close(results)
	for result := range results {
		if result.err != nil {
			if errors.Is(result.err, context.Canceled) || errors.Is(result.err, context.DeadlineExceeded) {
				return nil, result.err
			}
			continue
		}
		if !result.ok {
			continue
		}
		out = append(out, result.summary)
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].UpdatedAt.Equal(out[j].UpdatedAt) {
			return out[i].UpdatedAt.After(out[j].UpdatedAt)
		}
		return out[i].TeamID < out[j].TeamID
	})
	return out, nil
}

func (s *Store) LoadTeamCtx(ctx context.Context, teamID string) (Info, error) {
	if err := ctxErr(ctx); err != nil {
		return Info{}, err
	}
	return s.loadTeamNoCtx(teamID)
}

func (s *Store) LoadMembersCtx(ctx context.Context, teamID string) ([]Member, error) {
	if err := ctxErr(ctx); err != nil {
		return nil, err
	}
	return s.loadMembersNoCtx(teamID)
}

func (s *Store) LoadPolicyCtx(ctx context.Context, teamID string) (Policy, error) {
	if err := ctxErr(ctx); err != nil {
		return Policy{}, err
	}
	return s.loadPolicyNoCtx(teamID)
}

func (s *Store) LoadMembersSnapshotCtx(ctx context.Context, teamID string) ([]Member, time.Time, error) {
	if err := ctxErr(ctx); err != nil {
		return nil, time.Time{}, err
	}
	return s.loadMembersSnapshotNoCtx(teamID)
}

func (s *Store) LoadPolicySnapshotCtx(ctx context.Context, teamID string) (Policy, time.Time, error) {
	if err := ctxErr(ctx); err != nil {
		return Policy{}, time.Time{}, err
	}
	return s.loadPolicySnapshotNoCtx(teamID)
}

func (s *Store) LoadChannelSnapshotCtx(ctx context.Context, teamID, channelID string) (Channel, time.Time, error) {
	if err := ctxErr(ctx); err != nil {
		return Channel{}, time.Time{}, err
	}
	return s.loadChannelSnapshotNoCtx(teamID, channelID)
}

func (s *Store) LoadChannelConfigCtx(ctx context.Context, teamID, channelID string) (ChannelConfig, error) {
	if err := ctxErr(ctx); err != nil {
		return ChannelConfig{}, err
	}
	return s.loadChannelConfigNoCtx(teamID, channelID)
}

func (s *Store) LoadMessagesCtx(ctx context.Context, teamID, channelID string, limit int) ([]Message, error) {
	if err := ctxErr(ctx); err != nil {
		return nil, err
	}
	return s.loadMessagesNoCtx(teamID, channelID, limit)
}

func (s *Store) LoadAllMessagesCtx(ctx context.Context, teamID, channelID string) ([]Message, error) {
	return s.LoadMessagesCtx(ctx, teamID, channelID, 0)
}

func (s *Store) LoadChannelCtx(ctx context.Context, teamID, channelID string) (Channel, error) {
	if err := ctxErr(ctx); err != nil {
		return Channel{}, err
	}
	return s.loadChannelNoCtx(teamID, channelID)
}

func (s *Store) ListChannelsCtx(ctx context.Context, teamID string) ([]ChannelSummary, error) {
	if err := ctxErr(ctx); err != nil {
		return nil, err
	}
	return s.listChannelsNoCtx(teamID)
}

func (s *Store) ListChannelConfigsCtx(ctx context.Context, teamID string) ([]ChannelConfig, error) {
	if err := ctxErr(ctx); err != nil {
		return nil, err
	}
	return s.listChannelConfigsNoCtx(teamID)
}

func (s *Store) LoadTasksCtx(ctx context.Context, teamID string, limit int) ([]Task, error) {
	if err := ctxErr(ctx); err != nil {
		return nil, err
	}
	return s.loadTasksNoCtx(teamID, limit)
}

func (s *Store) LoadTaskCtx(ctx context.Context, teamID, taskID string) (Task, error) {
	if err := ctxErr(ctx); err != nil {
		return Task{}, err
	}
	return s.loadTaskNoCtx(teamID, taskID)
}

func (s *Store) LoadArtifactsCtx(ctx context.Context, teamID string, limit int) ([]Artifact, error) {
	if err := ctxErr(ctx); err != nil {
		return nil, err
	}
	return s.loadArtifactsNoCtx(teamID, limit)
}

func (s *Store) LoadArtifactCtx(ctx context.Context, teamID, artifactID string) (Artifact, error) {
	if err := ctxErr(ctx); err != nil {
		return Artifact{}, err
	}
	return s.loadArtifactNoCtx(teamID, artifactID)
}

func (s *Store) LoadHistoryCtx(ctx context.Context, teamID string, limit int) ([]ChangeEvent, error) {
	if err := ctxErr(ctx); err != nil {
		return nil, err
	}
	return s.loadHistoryNoCtx(teamID, limit)
}

func (s *Store) LoadWebhookConfigsCtx(ctx context.Context, teamID string) ([]PushNotificationConfig, error) {
	if err := ctxErr(ctx); err != nil {
		return nil, err
	}
	return s.loadWebhookConfigsNoCtx(teamID)
}

func (s *Store) ListArchivesCtx(ctx context.Context, teamID string) ([]ArchiveSnapshot, error) {
	if err := ctxErr(ctx); err != nil {
		return nil, err
	}
	return s.listArchivesNoCtx(teamID)
}

func (s *Store) LoadArchiveCtx(ctx context.Context, teamID, archiveID string) (ArchiveSnapshot, error) {
	if err := ctxErr(ctx); err != nil {
		return ArchiveSnapshot{}, err
	}
	return s.loadArchiveNoCtx(teamID, archiveID)
}

func (s *Store) LoadAgentCardCtx(ctx context.Context, teamID, agentID string) (AgentCard, error) {
	if err := ctxErr(ctx); err != nil {
		return AgentCard{}, err
	}
	return s.loadAgentCardNoCtx(teamID, agentID)
}

func (s *Store) ListAgentCardsCtx(ctx context.Context, teamID string) ([]AgentCard, error) {
	if err := ctxErr(ctx); err != nil {
		return nil, err
	}
	return s.listAgentCardsNoCtx(teamID)
}

func (s *Store) LoadTasksByContextCtx(ctx context.Context, teamID, contextID string) ([]Task, error) {
	if err := ctxErr(ctx); err != nil {
		return nil, err
	}
	return s.loadTasksByContextNoCtx(teamID, contextID)
}

func (s *Store) LoadTaskMessagesCtx(ctx context.Context, teamID, taskID string, limit int) ([]Message, error) {
	if err := ctxErr(ctx); err != nil {
		return nil, err
	}
	teamID = NormalizeTeamID(teamID)
	taskID = strings.TrimSpace(taskID)
	if teamID == "" {
		return nil, errors.New("empty team id")
	}
	if taskID == "" {
		return nil, errors.New("empty task id")
	}
	task, err := s.LoadTaskCtx(ctx, teamID, taskID)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	channelSummaries, err := s.ListChannelsCtx(ctx, teamID)
	if err != nil {
		return nil, err
	}
	preferred := []string{}
	if task.TaskID != "" {
		preferred = append(preferred, task.ChannelID)
		if task.ContextID != "" {
			tasksByContext, err := s.LoadTasksByContextCtx(ctx, teamID, task.ContextID)
			if err != nil {
				return nil, err
			}
			for _, item := range tasksByContext {
				preferred = append(preferred, item.ChannelID)
			}
		}
	}
	preferred = append(preferred, "main")
	channels := orderedChannelIDs(channelSummaries, preferred...)
	return loadMessagesMatchingChannelsCtx(ctx, channels, limit, func(message Message) bool {
		return taskIDMatches(message.StructuredData, taskID)
	}, func(channelID string) ([]Message, error) {
		return s.LoadAllMessagesCtx(ctx, teamID, channelID)
	})
}

func (s *Store) LoadMessagesByContextCtx(ctx context.Context, teamID, contextID string, limit int) ([]Message, error) {
	if err := ctxErr(ctx); err != nil {
		return nil, err
	}
	teamID = NormalizeTeamID(teamID)
	contextID = normalizeContextID(contextID)
	if teamID == "" {
		return nil, errors.New("empty team id")
	}
	if contextID == "" {
		return nil, errors.New("empty context id")
	}
	tasks, err := s.LoadTasksByContextCtx(ctx, teamID, contextID)
	if err != nil {
		return nil, err
	}
	channelSummaries, err := s.ListChannelsCtx(ctx, teamID)
	if err != nil {
		return nil, err
	}
	preferred := []string{"main"}
	for _, task := range tasks {
		preferred = append(preferred, task.ChannelID)
	}
	channels := orderedChannelIDs(channelSummaries, preferred...)
	return loadMessagesMatchingChannelsCtx(ctx, channels, limit, func(message Message) bool {
		return normalizeContextID(message.ContextID) == contextID || structuredDataContextID(message.StructuredData) == contextID
	}, func(channelID string) ([]Message, error) {
		return s.LoadAllMessagesCtx(ctx, teamID, channelID)
	})
}

func (s *Store) SaveMembersCtx(ctx context.Context, teamID string, members []Member) error {
	if err := ctxErr(ctx); err != nil {
		return err
	}
	return s.saveMembersNoCtx(teamID, members)
}

func (s *Store) SaveWebhookConfigsCtx(ctx context.Context, teamID string, configs []PushNotificationConfig) error {
	if err := ctxErr(ctx); err != nil {
		return err
	}
	return s.saveWebhookConfigsNoCtx(teamID, configs)
}

func (s *Store) SavePolicyCtx(ctx context.Context, teamID string, policy Policy) error {
	if err := ctxErr(ctx); err != nil {
		return err
	}
	return s.savePolicyNoCtx(teamID, policy)
}

func (s *Store) SaveChannelConfigCtx(ctx context.Context, teamID string, cfg ChannelConfig) error {
	if err := ctxErr(ctx); err != nil {
		return err
	}
	return s.saveChannelConfigNoCtx(teamID, cfg)
}

func (s *Store) AppendMessageCtx(ctx context.Context, teamID string, msg Message) error {
	if err := ctxErr(ctx); err != nil {
		return err
	}
	return s.appendMessageNoCtx(teamID, msg)
}

func (s *Store) SaveChannelCtx(ctx context.Context, teamID string, channel Channel) error {
	if err := ctxErr(ctx); err != nil {
		return err
	}
	return s.saveChannelNoCtx(teamID, channel)
}

func (s *Store) HideChannelCtx(ctx context.Context, teamID, channelID string) error {
	if err := ctxErr(ctx); err != nil {
		return err
	}
	return s.hideChannelNoCtx(teamID, channelID)
}

func (s *Store) AppendTaskCtx(ctx context.Context, teamID string, task Task) error {
	if err := ctxErr(ctx); err != nil {
		return err
	}
	return s.appendTaskNoCtx(teamID, task)
}

func (s *Store) SaveTaskCtx(ctx context.Context, teamID string, task Task) error {
	if err := ctxErr(ctx); err != nil {
		return err
	}
	return s.saveTaskNoCtx(teamID, task)
}

func (s *Store) DeleteTaskCtx(ctx context.Context, teamID, taskID string) error {
	if err := ctxErr(ctx); err != nil {
		return err
	}
	return s.deleteTaskNoCtx(teamID, taskID)
}

func (s *Store) AppendArtifactCtx(ctx context.Context, teamID string, artifact Artifact) error {
	if err := ctxErr(ctx); err != nil {
		return err
	}
	return s.appendArtifactNoCtx(teamID, artifact)
}

func (s *Store) SaveArtifactCtx(ctx context.Context, teamID string, artifact Artifact) error {
	if err := ctxErr(ctx); err != nil {
		return err
	}
	return s.saveArtifactNoCtx(teamID, artifact)
}

func (s *Store) DeleteArtifactCtx(ctx context.Context, teamID, artifactID string) error {
	if err := ctxErr(ctx); err != nil {
		return err
	}
	return s.deleteArtifactNoCtx(teamID, artifactID)
}

func (s *Store) AppendHistoryCtx(ctx context.Context, teamID string, event ChangeEvent) error {
	if err := ctxErr(ctx); err != nil {
		return err
	}
	return s.appendHistoryNoCtx(teamID, event)
}

func (s *Store) CreateManualArchiveCtx(ctx context.Context, teamID string, now time.Time) (*ArchiveSnapshot, error) {
	if err := ctxErr(ctx); err != nil {
		return nil, err
	}
	return s.createManualArchiveNoCtx(teamID, now)
}

func (s *Store) SaveAgentCardCtx(ctx context.Context, teamID string, card AgentCard) error {
	if err := ctxErr(ctx); err != nil {
		return err
	}
	return s.saveAgentCardNoCtx(teamID, card)
}
