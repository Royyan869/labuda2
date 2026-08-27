package application

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	authentity "github.com/labuda/backend/internal/identity/auth/entity"
	authrepo "github.com/labuda/backend/internal/identity/auth/infrastructure/repository"
	notificationentity "github.com/labuda/backend/internal/interaction/notification/entity"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

// LogoutCurrentSessionRequest captures the minimum state needed to revoke the
// current refresh session family and optionally deactivate the device's FCM
// token.
type LogoutCurrentSessionRequest struct {
	RefreshToken string
	FCMToken     string
	DeviceID     string
}

// LogoutCurrentSessionError is a typed, HTTP-mappable logout failure.
type LogoutCurrentSessionError struct {
	Code    string
	Status  int
	Message string
	Err     error
}

func (e *LogoutCurrentSessionError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Code, e.Err)
	}
	return e.Message
}

func (e *LogoutCurrentSessionError) Unwrap() error {
	return e.Err
}

// LogoutCurrentSessionResult carries non-fatal cleanup warnings.
type LogoutCurrentSessionResult struct {
	FCMWarning error
}

// LogoutAllSessionsRequest controls account-wide session revocation behavior.
type LogoutAllSessionsRequest struct {
	DeactivateFCMTokens *bool
}

// LogoutAllSessionsError is a typed, HTTP-mappable logout-all failure.
type LogoutAllSessionsError struct {
	Code    string
	Status  int
	Message string
	Err     error
}

func (e *LogoutAllSessionsError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Code, e.Err)
	}
	return e.Message
}

func (e *LogoutAllSessionsError) Unwrap() error {
	return e.Err
}

// LogoutAllSessionsResult carries non-fatal cleanup counts and warnings.
type LogoutAllSessionsResult struct {
	RevokedSessionsCount      int64
	DeactivatedFCMTokensCount int64
	FCMWarning                error
}

// SessionDeviceSummary is the safe session snapshot exposed to the user's
// future sessions UI.
type SessionDeviceSummary struct {
	FamilyID       uuid.UUID
	DeviceID       *string
	DeviceName     *string
	Platform       *string
	AppVersion     *string
	IssuedAt       time.Time
	LastUsedAt     *time.Time
	ExpiresAt      time.Time
	FCMTokenActive *bool
}

// RevokeSessionFamilyResult carries the outcome of a targeted family revoke.
type RevokeSessionFamilyResult struct {
	RevokedSessionsCount      int64
	DeactivatedFCMTokensCount int64
	FCMWarning                error
}

// SessionManagementError is a typed, HTTP-mappable sessions API failure.
type SessionManagementError struct {
	Code    string
	Status  int
	Message string
	Err     error
}

func (e *SessionManagementError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Code, e.Err)
	}
	return e.Message
}

func (e *SessionManagementError) Unwrap() error {
	return e.Err
}

type refreshTokenValidator interface {
	ValidateRefreshToken(tokenString string) (*Claims, error)
}

type logoutRefreshSessionRepository interface {
	FindByTokenHash(ctx context.Context, tx db.Tx, tokenHash string) (*authentity.RefreshSession, error)
	RevokeFamily(ctx context.Context, tx db.Tx, userID uuid.UUID, familyID uuid.UUID) error
	RevokeFamilyCount(ctx context.Context, tx db.Tx, userID uuid.UUID, familyID uuid.UUID) (int64, error)
	ListActiveByUser(ctx context.Context, tx db.Tx, userID uuid.UUID) ([]*authentity.RefreshSession, error)
	RevokeAllForUserCount(ctx context.Context, tx db.Tx, userID uuid.UUID) (int64, error)
}

type logoutFCMTokenRepository interface {
	GetActiveTokensByUser(ctx context.Context, tx interface{}, userID uuid.UUID) ([]*notificationentity.FCMToken, error)
	DeactivateByToken(ctx context.Context, tx interface{}, tokenString string) error
	DeactivateByUserAndDevice(ctx context.Context, tx interface{}, userID uuid.UUID, deviceID string) error
	DeactivateByUserAndDeviceCount(ctx context.Context, tx interface{}, userID uuid.UUID, deviceID string) (int64, error)
	DeactivateAllByUser(ctx context.Context, tx interface{}, userID uuid.UUID) (int64, error)
}

type pgxTxAdapter struct {
	tx pgx.Tx
}

func (a *pgxTxAdapter) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return a.tx.Exec(ctx, sql, args...)
}

func (a *pgxTxAdapter) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return a.tx.Query(ctx, sql, args...)
}

func (a *pgxTxAdapter) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return a.tx.QueryRow(ctx, sql, args...)
}

func (a *pgxTxAdapter) Commit(ctx context.Context) error {
	return a.tx.Commit(ctx)
}

func (a *pgxTxAdapter) Rollback(ctx context.Context) error {
	return a.tx.Rollback(ctx)
}

var _ db.Tx = (*pgxTxAdapter)(nil)

type poolTransactor struct {
	pool *pgxpool.Pool
}

// NewLogoutCurrentSessionTransactor adapts a pgx pool to the db.Transactor
// interface expected by the logout coordinator.
func NewLogoutCurrentSessionTransactor(pool *pgxpool.Pool) db.Transactor {
	return &poolTransactor{pool: pool}
}

func (p *poolTransactor) WithTx(ctx context.Context, fn func(tx db.Tx) error) error {
	if p == nil || p.pool == nil {
		return fmt.Errorf("logout transactor unavailable")
	}

	pgxTx, err := p.pool.Begin(ctx)
	if err != nil {
		return err
	}
	tx := &pgxTxAdapter{tx: pgxTx}
	defer tx.Rollback(ctx) //nolint:errcheck

	if err := fn(tx); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

var _ db.Transactor = (*poolTransactor)(nil)

// LogoutCurrentSessionService owns the current-session logout orchestration.
type LogoutCurrentSessionService struct {
	db                 db.Transactor
	tokenService       refreshTokenValidator
	refreshSessionRepo logoutRefreshSessionRepository
	fcmTokenRepo       logoutFCMTokenRepository
	log                *zap.Logger
}

// NewLogoutCurrentSessionService wires the logout coordinator.
func NewLogoutCurrentSessionService(
	database db.Transactor,
	tokenService refreshTokenValidator,
	refreshSessionRepo logoutRefreshSessionRepository,
	fcmTokenRepo logoutFCMTokenRepository,
	log *zap.Logger,
) *LogoutCurrentSessionService {
	if log == nil {
		log = zap.NewNop()
	}
	return &LogoutCurrentSessionService{
		db:                 database,
		tokenService:       tokenService,
		refreshSessionRepo: refreshSessionRepo,
		fcmTokenRepo:       fcmTokenRepo,
		log:                log,
	}
}

// LogoutCurrentSession revokes only the current refresh-session family.
func (s *LogoutCurrentSessionService) LogoutCurrentSession(
	ctx context.Context,
	userID uuid.UUID,
	req LogoutCurrentSessionRequest,
) error {
	rawToken := strings.TrimSpace(req.RefreshToken)
	if rawToken == "" {
		return &LogoutCurrentSessionError{
			Code:    "BAD_REQUEST",
			Status:  http.StatusBadRequest,
			Message: "refresh_token cannot be empty",
		}
	}

	claims, err := s.tokenService.ValidateRefreshToken(rawToken)
	if err != nil {
		s.log.Warn("Logout refresh token validation failed", zap.Error(err))
		return &LogoutCurrentSessionError{
			Code:    "INVALID_TOKEN",
			Status:  http.StatusUnauthorized,
			Message: "Invalid or expired refresh token",
			Err:     err,
		}
	}

	if claims.UserID == uuid.Nil {
		s.log.Warn("Logout refresh token missing user id")
		return &LogoutCurrentSessionError{
			Code:    "INVALID_TOKEN",
			Status:  http.StatusUnauthorized,
			Message: "Invalid refresh token claims",
		}
	}

	if claims.UserID != userID {
		s.log.Warn("Logout token user mismatch",
			zap.String("auth_user_id", userID.String()),
			zap.String("token_user_id", claims.UserID.String()),
		)
		return &LogoutCurrentSessionError{
			Code:    "TOKEN_USER_MISMATCH",
			Status:  http.StatusUnauthorized,
			Message: "Refresh token does not belong to the authenticated user",
		}
	}

	if s.db == nil {
		return &LogoutCurrentSessionError{
			Code:    "DATABASE_ERROR",
			Status:  http.StatusInternalServerError,
			Message: "Database error",
		}
	}

	tokenHash := authrepo.HashRefreshToken(rawToken)

	if err := s.db.WithTx(ctx, func(tx db.Tx) error {
		session, findErr := s.refreshSessionRepo.FindByTokenHash(ctx, tx, tokenHash)
		if findErr != nil {
			if errors.Is(findErr, authrepo.ErrSessionNotFound) {
				s.log.Warn("Logout refresh session not found",
					zap.String("user_id", userID.String()),
				)
				return &LogoutCurrentSessionError{
					Code:    "SESSION_NOT_FOUND",
					Status:  http.StatusUnauthorized,
					Message: "Refresh session not found",
				}
			}
			s.log.Error("Failed to load refresh session for logout", zap.Error(findErr))
			return &LogoutCurrentSessionError{
				Code:    "DATABASE_ERROR",
				Status:  http.StatusInternalServerError,
				Message: "Database error",
				Err:     findErr,
			}
		}

		if session.UserID != userID {
			s.log.Warn("Logout token hash belongs to another user",
				zap.String("auth_user_id", userID.String()),
				zap.String("session_user_id", session.UserID.String()),
			)
			return &LogoutCurrentSessionError{
				Code:    "TOKEN_USER_MISMATCH",
				Status:  http.StatusUnauthorized,
				Message: "Refresh token does not belong to the authenticated user",
			}
		}

		switch session.Status {
		case authentity.RefreshSessionStatusActive:
			if err := s.refreshSessionRepo.RevokeFamily(ctx, tx, userID, session.FamilyID); err != nil {
				s.log.Error("Failed to revoke refresh session family", zap.Error(err))
				return &LogoutCurrentSessionError{
					Code:    "DATABASE_ERROR",
					Status:  http.StatusInternalServerError,
					Message: "Database error",
					Err:     err,
				}
			}
			return nil

		case authentity.RefreshSessionStatusRevoked:
			return nil

		case authentity.RefreshSessionStatusConsumed, authentity.RefreshSessionStatusReused:
			s.log.Warn("Logout token is no longer current",
				zap.String("user_id", userID.String()),
				zap.String("status", string(session.Status)),
			)
			return &LogoutCurrentSessionError{
				Code:    "TOKEN_REUSE",
				Status:  http.StatusUnauthorized,
				Message: "Refresh token is no longer valid",
			}

		default:
			s.log.Warn("Logout token in unexpected refresh session state",
				zap.String("user_id", userID.String()),
				zap.String("status", string(session.Status)),
			)
			return &LogoutCurrentSessionError{
				Code:    "INVALID_TOKEN",
				Status:  http.StatusUnauthorized,
				Message: "Refresh token is no longer valid",
			}
		}
	}); err != nil {
		var logoutErr *LogoutCurrentSessionError
		if errors.As(err, &logoutErr) {
			return logoutErr
		}
		return &LogoutCurrentSessionError{
			Code:    "DATABASE_ERROR",
			Status:  http.StatusInternalServerError,
			Message: "Database error",
			Err:     err,
		}
	}

	if err := s.deactivateLogoutFCMToken(ctx, userID, req); err != nil {
		s.log.Warn("Logout FCM deactivation failed after session revoke",
			zap.Error(err),
			zap.String("user_id", userID.String()),
		)
	}

	return nil
}

// LogoutAllSessions revokes all active refresh sessions for the authenticated user.
//
// FCM token deactivation is best-effort and runs after the session revocation
// transaction commits, so a push-token cleanup failure cannot roll back the
// account-wide logout.
func (s *LogoutCurrentSessionService) LogoutAllSessions(
	ctx context.Context,
	userID uuid.UUID,
	req LogoutAllSessionsRequest,
) (*LogoutAllSessionsResult, error) {
	if userID == uuid.Nil {
		return nil, &LogoutAllSessionsError{
			Code:    "BAD_REQUEST",
			Status:  http.StatusBadRequest,
			Message: "user_id cannot be empty",
		}
	}

	if s.db == nil {
		return nil, &LogoutAllSessionsError{
			Code:    "DATABASE_ERROR",
			Status:  http.StatusInternalServerError,
			Message: "Database error",
		}
	}

	var revokedCount int64
	if err := s.db.WithTx(ctx, func(tx db.Tx) error {
		count, revokeErr := s.refreshSessionRepo.RevokeAllForUserCount(ctx, tx, userID)
		if revokeErr != nil {
			s.log.Error("Failed to revoke all refresh sessions", zap.Error(revokeErr))
			return &LogoutAllSessionsError{
				Code:    "DATABASE_ERROR",
				Status:  http.StatusInternalServerError,
				Message: "Database error",
				Err:     revokeErr,
			}
		}
		revokedCount = count
		return nil
	}); err != nil {
		var logoutErr *LogoutAllSessionsError
		if errors.As(err, &logoutErr) {
			return nil, logoutErr
		}
		return nil, &LogoutAllSessionsError{
			Code:    "DATABASE_ERROR",
			Status:  http.StatusInternalServerError,
			Message: "Database error",
			Err:     err,
		}
	}

	result := &LogoutAllSessionsResult{
		RevokedSessionsCount: revokedCount,
	}

	deactivateFCM := true
	if req.DeactivateFCMTokens != nil {
		deactivateFCM = *req.DeactivateFCMTokens
	}
	if deactivateFCM {
		count, err := s.deactivateAllLogoutFCMTokens(ctx, userID)
		if err != nil {
			result.FCMWarning = err
			s.log.Warn("Logout-all FCM deactivation failed after session revoke",
				zap.Error(err),
				zap.String("user_id", userID.String()),
			)
		} else {
			result.DeactivatedFCMTokensCount = count
		}
	}

	return result, nil
}

func (s *LogoutCurrentSessionService) deactivateLogoutFCMToken(
	ctx context.Context,
	userID uuid.UUID,
	req LogoutCurrentSessionRequest,
) error {
	if s.fcmTokenRepo == nil {
		return nil
	}

	token := strings.TrimSpace(req.FCMToken)
	deviceID := strings.TrimSpace(req.DeviceID)
	if token == "" && deviceID == "" {
		return nil
	}

	if s.db == nil {
		return fmt.Errorf("logout transactor unavailable")
	}

	var opErr error
	if err := s.db.WithTx(ctx, func(tx db.Tx) error {
		if token != "" {
			if err := s.fcmTokenRepo.DeactivateByToken(ctx, tx, token); err != nil {
				opErr = err
			}
		}
		if deviceID != "" {
			if err := s.fcmTokenRepo.DeactivateByUserAndDevice(ctx, tx, userID, deviceID); err != nil && opErr == nil {
				opErr = err
			}
		}
		return nil
	}); err != nil {
		return fmt.Errorf("commit fcm deactivation tx: %w", err)
	}

	return opErr
}

func (s *LogoutCurrentSessionService) deactivateAllLogoutFCMTokens(
	ctx context.Context,
	userID uuid.UUID,
) (int64, error) {
	if s.fcmTokenRepo == nil {
		return 0, nil
	}
	if s.db == nil {
		return 0, fmt.Errorf("logout transactor unavailable")
	}

	var count int64
	if err := s.db.WithTx(ctx, func(tx db.Tx) error {
		deactivatedCount, err := s.fcmTokenRepo.DeactivateAllByUser(ctx, tx, userID)
		if err != nil {
			return err
		}
		count = deactivatedCount
		return nil
	}); err != nil {
		return 0, fmt.Errorf("deactivate all fcm tokens: %w", err)
	}

	return count, nil
}

// ListActiveSessions returns the current user's active session/device snapshots.
//
// The result is de-duplicated by family_id so a rotated family appears only
// once, with the latest active row chosen as the canonical snapshot.
func (s *LogoutCurrentSessionService) ListActiveSessions(
	ctx context.Context,
	userID uuid.UUID,
) ([]*SessionDeviceSummary, error) {
	if userID == uuid.Nil {
		return nil, &SessionManagementError{
			Code:    "BAD_REQUEST",
			Status:  http.StatusBadRequest,
			Message: "user_id cannot be empty",
		}
	}
	if s.db == nil {
		return nil, &SessionManagementError{
			Code:    "DATABASE_ERROR",
			Status:  http.StatusInternalServerError,
			Message: "Database error",
		}
	}

	var sessions []*authentity.RefreshSession
	var fcmTokens []*notificationentity.FCMToken
	if err := s.db.WithTx(ctx, func(tx db.Tx) error {
		list, err := s.refreshSessionRepo.ListActiveByUser(ctx, tx, userID)
		if err != nil {
			s.log.Error("Failed to list active refresh sessions", zap.Error(err))
			return err
		}
		sessions = list

		if s.fcmTokenRepo != nil {
			tokens, tokenErr := s.fcmTokenRepo.GetActiveTokensByUser(ctx, tx, userID)
			if tokenErr != nil {
				s.log.Warn("Failed to enrich session list with FCM tokens", zap.Error(tokenErr))
			} else {
				fcmTokens = tokens
			}
		}
		return nil
	}); err != nil {
		var sessionErr *SessionManagementError
		if errors.As(err, &sessionErr) {
			return nil, sessionErr
		}
		return nil, &SessionManagementError{
			Code:    "DATABASE_ERROR",
			Status:  http.StatusInternalServerError,
			Message: "Database error",
			Err:     err,
		}
	}

	fcmByDevice := make(map[string]*notificationentity.FCMToken, len(fcmTokens))
	for _, token := range fcmTokens {
		if token == nil || token.DeviceID == nil {
			continue
		}
		deviceID := strings.TrimSpace(*token.DeviceID)
		if deviceID == "" {
			continue
		}
		fcmByDevice[deviceID] = token
	}

	result := make([]*SessionDeviceSummary, 0, len(sessions))
	seenFamilies := make(map[uuid.UUID]struct{}, len(sessions))
	for _, session := range sessions {
		if session == nil {
			continue
		}
		if _, seen := seenFamilies[session.FamilyID]; seen {
			continue
		}
		seenFamilies[session.FamilyID] = struct{}{}

		summary := &SessionDeviceSummary{
			FamilyID:   session.FamilyID,
			DeviceID:   session.DeviceID,
			DeviceName: session.DeviceName,
			Platform:   session.Platform,
			AppVersion: session.AppVersion,
			IssuedAt:   session.IssuedAt,
			ExpiresAt:  session.ExpiresAt,
		}
		if session.DeviceID != nil {
			if token := fcmByDevice[strings.TrimSpace(*session.DeviceID)]; token != nil {
				summary.FCMTokenActive = boolPtr(token.IsActive)
				summary.LastUsedAt = token.LastUsedAt
				summary.DeviceName = token.DeviceName
				platform := string(token.Platform)
				summary.Platform = &platform
				summary.AppVersion = token.AppVersion
			}
		}
		result = append(result, summary)
	}

	return result, nil
}

// RevokeSessionFamily revokes one active refresh family for the authenticated user.
//
// FCM deactivation is best-effort and only runs if the family snapshot
// includes a device ID we can safely target.
func (s *LogoutCurrentSessionService) RevokeSessionFamily(
	ctx context.Context,
	userID uuid.UUID,
	familyID uuid.UUID,
) (*RevokeSessionFamilyResult, error) {
	if userID == uuid.Nil || familyID == uuid.Nil {
		return nil, &SessionManagementError{
			Code:    "BAD_REQUEST",
			Status:  http.StatusBadRequest,
			Message: "user_id and family_id cannot be empty",
		}
	}
	if s.db == nil {
		return nil, &SessionManagementError{
			Code:    "DATABASE_ERROR",
			Status:  http.StatusInternalServerError,
			Message: "Database error",
		}
	}

	var session *authentity.RefreshSession
	var revokedCount int64
	if err := s.db.WithTx(ctx, func(tx db.Tx) error {
		list, err := s.refreshSessionRepo.ListActiveByUser(ctx, tx, userID)
		if err != nil {
			s.log.Error("Failed to load active sessions before family revoke", zap.Error(err))
			return &SessionManagementError{
				Code:    "DATABASE_ERROR",
				Status:  http.StatusInternalServerError,
				Message: "Database error",
				Err:     err,
			}
		}
		for _, candidate := range list {
			if candidate != nil && candidate.FamilyID == familyID {
				session = candidate
				break
			}
		}

		count, revokeErr := s.refreshSessionRepo.RevokeFamilyCount(ctx, tx, userID, familyID)
		if revokeErr != nil {
			s.log.Error("Failed to revoke refresh session family", zap.Error(revokeErr))
			return &SessionManagementError{
				Code:    "DATABASE_ERROR",
				Status:  http.StatusInternalServerError,
				Message: "Database error",
				Err:     revokeErr,
			}
		}
		revokedCount = count
		return nil
	}); err != nil {
		var revokeErr *SessionManagementError
		if errors.As(err, &revokeErr) {
			return nil, revokeErr
		}
		return nil, &SessionManagementError{
			Code:    "DATABASE_ERROR",
			Status:  http.StatusInternalServerError,
			Message: "Database error",
			Err:     err,
		}
	}

	result := &RevokeSessionFamilyResult{
		RevokedSessionsCount: revokedCount,
	}
	if session != nil && session.DeviceID != nil && strings.TrimSpace(*session.DeviceID) != "" {
		count, err := s.deactivateSessionDeviceTokens(ctx, userID, *session.DeviceID)
		if err != nil {
			result.FCMWarning = err
			s.log.Warn("Session family FCM deactivation failed after revoke",
				zap.Error(err),
				zap.String("user_id", userID.String()),
				zap.String("family_id", familyID.String()),
			)
		} else {
			result.DeactivatedFCMTokensCount = count
		}
	}

	return result, nil
}

func (s *LogoutCurrentSessionService) deactivateSessionDeviceTokens(
	ctx context.Context,
	userID uuid.UUID,
	deviceID string,
) (int64, error) {
	if s.fcmTokenRepo == nil {
		return 0, nil
	}
	if s.db == nil {
		return 0, fmt.Errorf("logout transactor unavailable")
	}

	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return 0, nil
	}

	var count int64
	if err := s.db.WithTx(ctx, func(tx db.Tx) error {
		deactivatedCount, err := s.fcmTokenRepo.DeactivateByUserAndDeviceCount(ctx, tx, userID, deviceID)
		if err != nil {
			return err
		}
		count = deactivatedCount
		return nil
	}); err != nil {
		return 0, fmt.Errorf("deactivate session device tokens: %w", err)
	}

	return count, nil
}

func boolPtr(v bool) *bool {
	return &v
}


