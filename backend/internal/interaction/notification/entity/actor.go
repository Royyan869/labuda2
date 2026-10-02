package entity

import (
	"errors"
	"fmt"

	"github.com/google/uuid"
)

// ActorKind is the canonical discriminator of a notification's cause.
//
// It is the single authority for the three business truths that the legacy
// uuid.Nil sentinel collapsed into one unpersistable value:
//
//	ActorKindUser       — a human user caused it (identity visible)
//	ActorKindSystem     — the platform caused it (no human exists)
//	ActorKindAnonymized — a human caused it, but block policy hides the
//	                      identity from the recipient; Display carries the
//	                      role label ("Penjual", "Admin", ...)
type ActorKind string

const (
	ActorKindUser       ActorKind = "user"
	ActorKindSystem     ActorKind = "system"
	ActorKindAnonymized ActorKind = "anonymized"
)

// ErrInvalidActor is returned when an actor cannot be represented.
var ErrInvalidActor = errors.New("notification: invalid actor")

// Actor is the canonical cause of a notification.
//
// The type makes the legacy defect unrepresentable: there is no nil-uuid
// "user"; a missing user id IS "no human actor" and collapses to the system
// actor. System and anonymized actors carry no user id by construction, so a
// sentinel can never reach persistence — or the users FK — again.
//
// The zero value is invalid; use the constructors.
type Actor struct {
	kind    ActorKind
	userID  uuid.UUID
	display string
}

// UserActor returns the actor for a human user. A nil user id means there is
// no human actor, so it collapses to the system actor — the only truthful
// representation.
func UserActor(userID uuid.UUID) Actor {
	if userID == uuid.Nil {
		return SystemActor()
	}
	return Actor{kind: ActorKindUser, userID: userID}
}

// SystemActor returns the actor for platform-caused notifications
// (timers, workers, gateways) that have no human actor.
func SystemActor() Actor {
	return Actor{kind: ActorKindSystem}
}

// AnonymizedActor returns the actor for a human whose identity is hidden from
// the recipient by block policy. Display is the role label the UI may show.
func AnonymizedActor(display string) Actor {
	return Actor{kind: ActorKindAnonymized, display: display}
}

// ParseUserActor resolves an optional actor id from an event payload.
//
// Legacy events emitted before an actor field existed carry none; such events
// are system-caused: never dropped for commerce/moderation, never persisted
// as a nil user identity.
func ParseUserActor(raw string) Actor {
	id, err := uuid.Parse(raw)
	if err != nil {
		return SystemActor()
	}
	return UserActor(id)
}

// NewActor reconstructs an actor from persisted columns. It is the storage
// authority: it rejects exactly the shapes the schema forbids.
func NewActor(kind ActorKind, userID *uuid.UUID, display string) (Actor, error) {
	var actor Actor
	switch kind {
	case ActorKindUser:
		if userID == nil || *userID == uuid.Nil {
			return Actor{}, fmt.Errorf("%w: kind %q requires a non-nil user id", ErrInvalidActor, kind)
		}
		actor = Actor{kind: kind, userID: *userID}
	case ActorKindSystem, ActorKindAnonymized:
		if userID != nil {
			return Actor{}, fmt.Errorf("%w: kind %q must not carry a user id", ErrInvalidActor, kind)
		}
		actor = Actor{kind: kind, display: display}
	default:
		return Actor{}, fmt.Errorf("%w: unknown kind %q", ErrInvalidActor, kind)
	}
	return actor, nil
}

// Kind returns the actor kind.
func (a Actor) Kind() ActorKind { return a.kind }

// IsUser reports whether a real, visible human caused the notification.
func (a Actor) IsUser() bool { return a.kind == ActorKindUser && a.userID != uuid.Nil }

// UserID returns the acting user id; uuid.Nil for system and anonymized.
func (a Actor) UserID() uuid.UUID { return a.userID }

// Display returns the role label for anonymized actors ("" otherwise).
func (a Actor) Display() string { return a.display }

// Validate reports whether the actor is representable and persistable.
func (a Actor) Validate() error {
	switch a.kind {
	case ActorKindUser:
		if a.userID == uuid.Nil {
			return fmt.Errorf("%w: kind %q requires a non-nil user id", ErrInvalidActor, a.kind)
		}
	case ActorKindSystem, ActorKindAnonymized:
		if a.userID != uuid.Nil {
			return fmt.Errorf("%w: kind %q must not carry a user id", ErrInvalidActor, a.kind)
		}
	default:
		return fmt.Errorf("%w: unknown kind %q", ErrInvalidActor, a.kind)
	}
	return nil
}

// StorageUserID is the value bound to notifications.actor_id: the user id for
// visible humans, SQL NULL for system and anonymized actors.
func (a Actor) StorageUserID() any {
	if a.IsUser() {
		return a.userID
	}
	return nil
}

// String renders the actor for logs.
func (a Actor) String() string {
	switch a.kind {
	case ActorKindUser:
		return "user:" + a.userID.String()
	case ActorKindAnonymized:
		return "anonymized:" + a.display
	case ActorKindSystem:
		return "system"
	default:
		return "invalid-actor"
	}
}
