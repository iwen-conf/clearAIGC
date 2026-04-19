package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/iwen-conf/Naturalize/internal/domain"
)

type TrackingPublisher struct {
	inner domain.ProgressPublisher
	state domain.SessionStateRepository
}

func NewTrackingPublisher(inner domain.ProgressPublisher, state domain.SessionStateRepository) *TrackingPublisher {
	return &TrackingPublisher{inner: inner, state: state}
}

func (p *TrackingPublisher) Publish(ctx context.Context, sessionID uuid.UUID, event domain.ProgressEvent) error {
	persistErr := p.recordEvent(ctx, sessionID, event)
	publishErr := p.inner.Publish(ctx, sessionID, event)
	if publishErr != nil {
		return publishErr
	}
	return persistErr
}

func (p *TrackingPublisher) Subscribe(ctx context.Context, sessionID uuid.UUID) (<-chan domain.ProgressEvent, func(), error) {
	return p.inner.Subscribe(ctx, sessionID)
}

func (p *TrackingPublisher) recordEvent(ctx context.Context, sessionID uuid.UUID, event domain.ProgressEvent) error {
	if p.state == nil {
		return nil
	}

	switch event.Type {
	case "progress":
		payload, err := decodeEventData[domain.SessionProgressSnapshot](event.Data)
		if err != nil {
			return err
		}
		payload.SessionID = sessionID
		if err := p.state.UpsertProgress(ctx, &payload); err != nil {
			return err
		}
		return p.state.AppendTimeline(ctx, &domain.SessionTimelineEntry{
			SessionID: sessionID,
			Round:     payload.Round,
			Tone:      domain.TimelineToneNeutral,
			Title:     fmt.Sprintf("第 %d 轮 · %s", payload.Round, payload.Phase),
			Detail:    fmt.Sprintf("%d/%d 片段已完成 · %s", payload.CompletedChunks, payload.TotalChunks, payload.ProviderUsed),
		})
	case "quality_alert":
		payload, err := decodeEventData[struct {
			Reason string `json:"reason"`
		}](event.Data)
		if err != nil {
			return err
		}
		round := 0
		if existing, getErr := p.state.GetProgress(ctx, sessionID); getErr == nil {
			round = existing.Round
			existing.Phase = "quality-alert"
			if err := p.state.UpsertProgress(ctx, existing); err != nil {
				return err
			}
		}
		return p.state.AppendTimeline(ctx, &domain.SessionTimelineEntry{
			SessionID: sessionID,
			Round:     round,
			Tone:      domain.TimelineToneWarning,
			Title:     "质量检查提示",
			Detail:    payload.Reason,
		})
	case "recovery":
		payload, err := decodeEventData[struct {
			ChunkID   string `json:"chunkId"`
			Success   bool   `json:"success"`
			Steps     int    `json:"steps"`
			TokenCost int    `json:"tokenCost"`
		}](event.Data)
		if err != nil {
			return err
		}
		round := 0
		if existing, getErr := p.state.GetProgress(ctx, sessionID); getErr == nil {
			round = existing.Round
			existing.Phase = "recovery"
			if err := p.state.UpsertProgress(ctx, existing); err != nil {
				return err
			}
		}
		title := "修复未通过"
		detail := fmt.Sprintf("片段 %s 无法自动确认", payload.ChunkID)
		tone := domain.TimelineToneWarning
		if payload.Success {
			title = "片段已修复"
			detail = fmt.Sprintf("经过 %d 次尝试通过 · 消耗 %d tokens", payload.Steps, payload.TokenCost)
			tone = domain.TimelineToneSuccess
		}
		return p.state.AppendTimeline(ctx, &domain.SessionTimelineEntry{
			SessionID: sessionID,
			Round:     round,
			Tone:      tone,
			Title:     title,
			Detail:    detail,
		})
	case "complete":
		payload, err := decodeEventData[struct {
			SessionID       uuid.UUID `json:"sessionId"`
			Round           int       `json:"round"`
			ChunkCount      int       `json:"chunkCount"`
			PassedChunks    int       `json:"passedChunks"`
			RecoveredChunks int       `json:"recoveredChunks"`
			TotalTokens     int64     `json:"totalTokens"`
			ProviderUsed    string    `json:"providerUsed"`
		}](event.Data)
		if err != nil {
			return err
		}
		if err := p.state.UpsertProgress(ctx, &domain.SessionProgressSnapshot{
			SessionID:       sessionID,
			Round:           payload.Round,
			Phase:           "complete",
			CompletedChunks: payload.ChunkCount,
			TotalChunks:     payload.ChunkCount,
			Percent:         100,
			ProviderUsed:    payload.ProviderUsed,
		}); err != nil {
			return err
		}
		return p.state.AppendTimeline(ctx, &domain.SessionTimelineEntry{
			SessionID: sessionID,
			Round:     payload.Round,
			Tone:      domain.TimelineToneSuccess,
			Title:     fmt.Sprintf("第 %d 轮完成", payload.Round),
			Detail:    fmt.Sprintf("%d/%d 片段就绪 · 消耗 %d tokens", payload.PassedChunks+payload.RecoveredChunks, payload.ChunkCount, payload.TotalTokens),
		})
	case "paused":
		payload, err := decodeEventData[struct {
			Round           int    `json:"round"`
			CompletedChunks int    `json:"completedChunks"`
			TotalChunks     int    `json:"totalChunks"`
			Reason          string `json:"reason"`
		}](event.Data)
		if err != nil {
			return err
		}
		snapshot := &domain.SessionProgressSnapshot{
			SessionID:       sessionID,
			Round:           payload.Round,
			Phase:           "paused",
			CompletedChunks: payload.CompletedChunks,
			TotalChunks:     payload.TotalChunks,
		}
		if payload.TotalChunks > 0 {
			snapshot.Percent = float64(payload.CompletedChunks) / float64(payload.TotalChunks) * 100
		} else if existing, getErr := p.state.GetProgress(ctx, sessionID); getErr == nil {
			snapshot.CompletedChunks = existing.CompletedChunks
			snapshot.TotalChunks = existing.TotalChunks
			snapshot.Percent = existing.Percent
			snapshot.ProviderUsed = existing.ProviderUsed
			snapshot.ChunkID = existing.ChunkID
			snapshot.ParagraphIndex = existing.ParagraphIndex
			snapshot.ChunkIndex = existing.ChunkIndex
		}
		if err := p.state.UpsertProgress(ctx, snapshot); err != nil {
			return err
		}
		detail := "您已手动暂停本次处理。"
		if payload.Reason == "provider_unavailable" {
			detail = "模型服务暂时不可用。"
		}
		return p.state.AppendTimeline(ctx, &domain.SessionTimelineEntry{
			SessionID: sessionID,
			Round:     payload.Round,
			Tone:      domain.TimelineToneWarning,
			Title:     "处理已暂停",
			Detail:    detail,
		})
	case "error":
		payload, err := decodeEventData[struct {
			Message string `json:"message"`
		}](event.Data)
		if err != nil {
			return err
		}
		snapshot := &domain.SessionProgressSnapshot{
			SessionID: sessionID,
			Phase:     "error",
		}
		round := 0
		if existing, getErr := p.state.GetProgress(ctx, sessionID); getErr == nil {
			round = existing.Round
			snapshot.Round = existing.Round
			snapshot.CompletedChunks = existing.CompletedChunks
			snapshot.TotalChunks = existing.TotalChunks
			snapshot.Percent = existing.Percent
			snapshot.ProviderUsed = existing.ProviderUsed
			snapshot.ChunkID = existing.ChunkID
			snapshot.ParagraphIndex = existing.ParagraphIndex
			snapshot.ChunkIndex = existing.ChunkIndex
		}
		if err := p.state.UpsertProgress(ctx, snapshot); err != nil {
			return err
		}
		return p.state.AppendTimeline(ctx, &domain.SessionTimelineEntry{
			SessionID: sessionID,
			Round:     round,
			Tone:      domain.TimelineToneError,
			Title:     "处理出错",
			Detail:    payload.Message,
		})
	default:
		return nil
	}
}

func decodeEventData[T any](data any) (T, error) {
	var payload T
	encoded, err := json.Marshal(data)
	if err != nil {
		return payload, err
	}
	if err := json.Unmarshal(encoded, &payload); err != nil {
		return payload, err
	}
	return payload, nil
}
