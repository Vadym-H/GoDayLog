package services

import (
	"context"
	"log/slog"

	"github.com/Vadym-H/GoDayLog/internal/domain"
)

type ActivityLogReader interface {
	GetActivityLog(ctx context.Context, activityID, userID string) (domain.ActivityLog, error)
	GetActivityLogs(ctx context.Context, userID string, limit, offset int) ([]domain.ActivityLog, error)
	GetActivityLogsByMessage(ctx context.Context, messageID, userID string) ([]domain.ActivityLog, error)
}

type ActivityLogMutator interface {
	SoftDeleteActivityLog(ctx context.Context, activityID, userID string) error
	UpdateActivityLog(ctx context.Context, activityID, userID string, fields domain.UpdateActivityFields) error
}

type MessageReader interface {
	GetMessage(ctx context.Context, messageID, userID string) (domain.Message, error)
}

type ActivityLogIDResolver interface {
	GetUserID(ctx context.Context, id domain.Identity) (string, error)
}

type ActivityLogService struct {
	log        *slog.Logger
	logReader  ActivityLogReader
	logMutator ActivityLogMutator
	msgReader  MessageReader
	idResolver ActivityLogIDResolver
}

func NewActivityLogService(
	log *slog.Logger,
	logReader ActivityLogReader,
	logMutator ActivityLogMutator,
	msgReader MessageReader,
	idResolver ActivityLogIDResolver,
) *ActivityLogService {
	return &ActivityLogService{
		log:        log,
		logReader:  logReader,
		logMutator: logMutator,
		msgReader:  msgReader,
		idResolver: idResolver,
	}
}

func (s *ActivityLogService) GetHistory(ctx context.Context, id domain.Identity, limit, offset int) ([]domain.ActivityLog, error) {
	userID, err := s.idResolver.GetUserID(ctx, id)
	if err != nil {
		return nil, err
	}
	return s.logReader.GetActivityLogs(ctx, userID, limit, offset)
}

func (s *ActivityLogService) GetMessageWithActivities(ctx context.Context, id domain.Identity, messageID string) (domain.MessageWithActivities, error) {
	userID, err := s.idResolver.GetUserID(ctx, id)
	if err != nil {
		return domain.MessageWithActivities{}, err
	}

	msg, err := s.msgReader.GetMessage(ctx, messageID, userID)
	if err != nil {
		return domain.MessageWithActivities{}, err
	}

	activities, err := s.logReader.GetActivityLogsByMessage(ctx, messageID, userID)
	if err != nil {
		return domain.MessageWithActivities{}, err
	}

	return domain.MessageWithActivities{Message: msg, Activities: activities}, nil
}

func (s *ActivityLogService) DeleteActivity(ctx context.Context, id domain.Identity, activityID string) error {
	userID, err := s.idResolver.GetUserID(ctx, id)
	if err != nil {
		return err
	}
	return s.logMutator.SoftDeleteActivityLog(ctx, activityID, userID)
}

func (s *ActivityLogService) UpdateActivity(ctx context.Context, id domain.Identity, activityID string, fields domain.UpdateActivityFields) (domain.ActivityLog, error) {
	userID, err := s.idResolver.GetUserID(ctx, id)
	if err != nil {
		return domain.ActivityLog{}, err
	}

	if err := s.logMutator.UpdateActivityLog(ctx, activityID, userID, fields); err != nil {
		return domain.ActivityLog{}, err
	}

	return s.logReader.GetActivityLog(ctx, activityID, userID)
}
