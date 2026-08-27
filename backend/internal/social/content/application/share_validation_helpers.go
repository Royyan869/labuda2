package application

import (
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/labuda/backend/internal/social/content/entity"
)

func validateShareTargetType(targetType entity.ShareTargetType) error {
	if !targetType.IsValid() {
		return fmt.Errorf("invalid share target type: %s", targetType)
	}
	return nil
}

func validateShareTargetEnvelope(
	targetType entity.ShareTargetType,
	targetID string,
) error {
	if err := validateShareTargetType(targetType); err != nil {
		return err
	}

	if targetID == "" {
		return fmt.Errorf("share target id is required")
	}

	return nil
}

func shareTargetNotFoundError(targetKind, targetID string, err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("%s not found: %s", targetKind, targetID)
	}
	return fmt.Errorf("failed to validate %s: %w", targetKind, err)
}

func shareTargetDeletedError(targetKind, targetID string) error {
	return fmt.Errorf("cannot share deleted %s: %s", targetKind, targetID)
}

func shareTargetStatusError(targetKind, status, targetID string) error {
	return fmt.Errorf("cannot share %s in status %q: %s", targetKind, status, targetID)
}
