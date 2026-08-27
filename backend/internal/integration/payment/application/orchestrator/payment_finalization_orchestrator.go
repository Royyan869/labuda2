package orchestrator

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

// PaymentFinalizationOrchestrator manages payment finalization routing
// to appropriate handlers based on reference type.
//
// This implements the Registry pattern where handlers are registered
// by their reference type and dispatched accordingly.
type PaymentFinalizationOrchestrator struct {
	handlers map[string]PaymentReferenceHandler
}

// NewPaymentFinalizationOrchestrator creates a new orchestrator with registered handlers.
//
// Each handler is registered by its ReferenceType(). If multiple handlers
// claim the same reference type, the last one wins.
func NewPaymentFinalizationOrchestrator(
	handlers []PaymentReferenceHandler,
) *PaymentFinalizationOrchestrator {
	registry := make(map[string]PaymentReferenceHandler)

	for _, h := range handlers {
		registry[h.ReferenceType()] = h
	}

	return &PaymentFinalizationOrchestrator{
		handlers: registry,
	}
}

// HandleCompleted routes payment completion to the appropriate handler.
//
// Returns an error if no handler is registered for the reference type.
func (o *PaymentFinalizationOrchestrator) HandleCompleted(
	ctx context.Context,
	referenceType string,
	paymentID uuid.UUID,
) error {
	handler, ok := o.handlers[referenceType]
	if !ok {
		return errors.New("no handler registered for reference type: " + referenceType)
	}

	return handler.HandlePaymentCompleted(ctx, paymentID)
}

// HandleExpired routes payment expiry to the appropriate handler.
//
// Returns an error if no handler is registered for the reference type.
func (o *PaymentFinalizationOrchestrator) HandleExpired(
	ctx context.Context,
	referenceType string,
	paymentID uuid.UUID,
) error {
	handler, ok := o.handlers[referenceType]
	if !ok {
		return errors.New("no handler registered for reference type: " + referenceType)
	}

	return handler.HandlePaymentExpired(ctx, paymentID)
}

// HandleRefunded routes payment refund to the appropriate handler.
//
// Returns an error if no handler is registered for the reference type.
func (o *PaymentFinalizationOrchestrator) HandleRefunded(
	ctx context.Context,
	referenceType string,
	paymentID uuid.UUID,
) error {
	handler, ok := o.handlers[referenceType]
	if !ok {
		return errors.New("no handler registered for reference type: " + referenceType)
	}

	return handler.HandlePaymentRefunded(ctx, paymentID)
}


