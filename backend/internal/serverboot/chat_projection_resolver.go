package serverboot

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/governance/viewercontext"
	chatApp "github.com/labuda/backend/internal/interaction/chat/application"
	chatEntity "github.com/labuda/backend/internal/interaction/chat/entity"
	"github.com/labuda/backend/internal/pkg/blockcheck"
	"github.com/labuda/backend/pkg/db"
)

type profileProjectionBatchResolver struct {
	db profileProjectionDB
}

type profileProjectionDB interface {
	WithTx(ctx context.Context, fn func(db.Tx) error) error
}

func newProfileProjectionBatchResolver(database profileProjectionDB) *profileProjectionBatchResolver {
	return &profileProjectionBatchResolver{db: database}
}

var _ chatApp.ResourceProjectionResolver = (*profileProjectionBatchResolver)(nil)

func (r *profileProjectionBatchResolver) ResolveResourceProjections(
	ctx context.Context,
	viewerID uuid.UUID,
	occurrences map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence,
) (map[uuid.UUID]*chatApp.ResourceProjection, error) {
	return r.ResolveProfiles(ctx, viewerID, occurrences)
}

// ResolveProfiles resolves only profile occurrences. Unsupported occurrence
// types are rejected rather than silently omitted or tombstoned.
func (r *profileProjectionBatchResolver) ResolveProfiles(
	ctx context.Context,
	viewerID uuid.UUID,
	occurrences map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence,
) (map[uuid.UUID]*chatApp.ResourceProjection, error) {
	if len(occurrences) == 0 {
		return map[uuid.UUID]*chatApp.ResourceProjection{}, nil
	}
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("chat: profile projection resolver not configured")
	}

	profileIDs := make([]uuid.UUID, 0, len(occurrences))
	seenProfileIDs := make(map[uuid.UUID]struct{}, len(occurrences))
	occurrenceByMsgID := make(map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence, len(occurrences))

	for messageID, occ := range occurrences {
		if occ == nil {
			return nil, fmt.Errorf("chat: nil occurrence for message %s", messageID)
		}
		if !occ.ResourceType().IsValid() {
			return nil, fmt.Errorf("chat: malformed occurrence identity for message %s", messageID)
		}
		if occ.ResourceType() != chatEntity.ResourceOccurrenceResourceTypeProfile {
			return nil, fmt.Errorf("chat: unsupported resource type %q in profile resolver", occ.ResourceType())
		}
		if occ.ProfileSourceID == nil {
			return nil, fmt.Errorf("chat: profile occurrence for message %s requires non-nil profile source id", messageID)
		}

		if _, seen := seenProfileIDs[*occ.ProfileSourceID]; !seen {
			profileIDs = append(profileIDs, *occ.ProfileSourceID)
			seenProfileIDs[*occ.ProfileSourceID] = struct{}{}
		}
		occurrenceByMsgID[messageID] = occ
	}

	if len(profileIDs) == 0 {
		return map[uuid.UUID]*chatApp.ResourceProjection{}, nil
	}

	result := make(map[uuid.UUID]*chatApp.ResourceProjection, len(occurrences))
	err := r.db.WithTx(ctx, func(tx db.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT
				u.id,
				COALESCE(up.username, '') AS username,
				up.avatar_url,
				sp.store_name,
				u.account_status,
				u.deleted_at,
				(sp.user_id IS NOT NULL) AS is_seller
			FROM users u
			JOIN user_profiles up ON up.user_id = u.id
			LEFT JOIN seller_profiles sp ON sp.user_id = u.id
			WHERE u.id = ANY($1)
		`, profileIDs)
		if err != nil {
			return fmt.Errorf("chat: profile source batch query failed: %w", err)
		}
		defer rows.Close()

		type profileSourceRow struct {
			userID        uuid.UUID
			username      string
			avatarURL     sql.NullString
			storeName     sql.NullString
			accountStatus string
			deletedAt     sql.NullTime
			isSeller      bool
		}

		byID := make(map[uuid.UUID]profileSourceRow, len(profileIDs))
		for rows.Next() {
			var row profileSourceRow
			if err := rows.Scan(
				&row.userID,
				&row.username,
				&row.avatarURL,
				&row.storeName,
				&row.accountStatus,
				&row.deletedAt,
				&row.isSeller,
			); err != nil {
				return fmt.Errorf("chat: profile source batch scan failed: %w", err)
			}
			byID[row.userID] = row
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("chat: profile source batch iteration failed: %w", err)
		}

		blockedSet, err := blockcheck.BlockedSet(ctx, tx, viewerID, profileIDs)
		if err != nil {
			return fmt.Errorf("chat: profile block batch query failed: %w", err)
		}

		for messageID, occ := range occurrenceByMsgID {
			sourceID := occ.SourceID()
			row, ok := byID[sourceID]
			if !ok {
				return fmt.Errorf("chat: profile source row missing for %s", sourceID)
			}

			lifecycle := viewercontext.CoarsenLifecycle(row.accountStatus, row.deletedAt.Valid)
			blocked := viewerID != uuid.Nil && viewerID != sourceID && blockedSet[sourceID]

			if lifecycle != viewercontext.PublicLifecycleStateActive || blocked {
				proj, err := chatApp.NewTombstoneProjection(chatEntity.ResourceOccurrenceResourceTypeProfile)
				if err != nil {
					return err
				}
				result[messageID] = &proj
				continue
			}

			var avatarURL *string
			if row.avatarURL.Valid && row.avatarURL.String != "" {
				v := row.avatarURL.String
				avatarURL = &v
			}

			var storeName *string
			if row.storeName.Valid && row.storeName.String != "" {
				v := row.storeName.String
				storeName = &v
			}

			payload := chatApp.ProfileLivePayload{
				Username:  row.username,
				AvatarURL: avatarURL,
				StoreName: storeName,
				IsSeller:  row.isSeller,
				Lifecycle: string(lifecycle),
			}

			proj, err := chatApp.NewLiveProjection(
				chatEntity.ResourceOccurrenceResourceTypeProfile,
				sourceID,
				payload,
				chatApp.ProjectionViewerCapabilities{
					CanView:            true,
					CanInteract:        false,
					BlockedByTombstone: false,
				},
				nil,
			)
			if err != nil {
				return err
			}
			result[messageID] = &proj
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}
