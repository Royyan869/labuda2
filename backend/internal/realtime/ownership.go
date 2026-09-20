package realtime

// OwnedOutboxEventTypes is the canonical ownership declaration for the realtime
// consumer.
//
// OWNERSHIP INVARIANT — ONE OUTBOX EVENT TYPE = ONE OWNING CONSUMER:
// every claimable outbox event type has exactly one owner. The types listed here
// are owned by the realtime worker; their single durable effect is WebSocket
// delivery. They MUST NOT be claimed by the outbox worker, which owns every
// other event type.
//
// The set is the single source of truth consumed by:
//   - the realtime worker, as its fetch/claim include scope;
//   - the outbox worker, as its fetch/claim exclude scope.
//
// Moving ownership of an event type to the realtime worker is done ONLY by
// adding it here. An event type listed here must never be registered as an
// outbox dispatcher handler (that would be duplicate ownership).
var OwnedOutboxEventTypes = []string{
	EventTypeChatMessageSent,
	EventTypeChatRoomCreated,
	EventTypeChatRoomUpdated,
}
