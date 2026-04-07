package team

import (
	"context"
	"sort"
)

func loadMessagesMatchingChannels(teamID string, channelIDs []string, limit int, match func(Message) bool, loader func(string) ([]Message, error)) ([]Message, error) {
	matched := make([]Message, 0)
	for _, channelID := range channelIDs {
		messages, err := loader(channelID)
		if err != nil {
			return nil, err
		}
		for _, message := range messages {
			if match(message) {
				matched = append(matched, message)
			}
		}
	}
	sort.SliceStable(matched, func(i, j int) bool {
		if !matched[i].CreatedAt.Equal(matched[j].CreatedAt) {
			return matched[i].CreatedAt.After(matched[j].CreatedAt)
		}
		return matched[i].MessageID > matched[j].MessageID
	})
	if limit > 0 && len(matched) > limit {
		matched = append([]Message(nil), matched[:limit]...)
	}
	return matched, nil
}

func loadMessagesMatchingChannelsCtx(ctx context.Context, channelIDs []string, limit int, match func(Message) bool, loader func(string) ([]Message, error)) ([]Message, error) {
	matched := make([]Message, 0)
	for _, channelID := range channelIDs {
		if err := ctxErr(ctx); err != nil {
			return nil, err
		}
		messages, err := loader(channelID)
		if err != nil {
			return nil, err
		}
		for _, message := range messages {
			if err := ctxErr(ctx); err != nil {
				return nil, err
			}
			if match(message) {
				matched = append(matched, message)
			}
		}
	}
	sort.SliceStable(matched, func(i, j int) bool {
		if !matched[i].CreatedAt.Equal(matched[j].CreatedAt) {
			return matched[i].CreatedAt.After(matched[j].CreatedAt)
		}
		return matched[i].MessageID > matched[j].MessageID
	})
	if limit > 0 && len(matched) > limit {
		matched = append([]Message(nil), matched[:limit]...)
	}
	return matched, nil
}

func orderedChannelIDs(summaries []ChannelSummary, preferred ...string) []string {
	seen := make(map[string]struct{}, len(summaries)+len(preferred))
	out := make([]string, 0, len(summaries)+len(preferred))
	add := func(channelID string) {
		channelID = normalizeChannelID(channelID)
		if channelID == "" {
			return
		}
		if _, ok := seen[channelID]; ok {
			return
		}
		seen[channelID] = struct{}{}
		out = append(out, channelID)
	}
	for _, channelID := range preferred {
		add(channelID)
	}
	for _, summary := range summaries {
		add(summary.ChannelID)
	}
	return out
}
