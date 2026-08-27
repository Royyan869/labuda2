# Attachment Schema V2 - Strict Type-Safe Design

## CLEAN BREAK - NO LEGACY

This is a complete redesign of the attachment system. No backward compatibility.

## Canonical Structure

All attachments follow this structure:

```json
{
  "type": "string",
  "data": { ... }
}
```

## Valid Types

### Object References (cross-domain canonical references)

#### 1. reference
```json
{
  "type": "reference",
  "data": {
    "target_type": "string (required: listing|auction|post|request)",
    "target_id": "string (required)",
    "preview": {
      "title": "string (required)",
      "imageUrl": "string (optional)",
      "isAvailable": "boolean (optional)",
      "isSold": "boolean (optional)",
      "isClosed": "boolean (optional)",
      "isDeleted": "boolean (optional)"
    }
  }
}
```

### Workflow Payloads (domain-specific business state)

#### 2. negotiation_offer
```json
{
  "type": "negotiation_offer",
  "data": {
    "negotiation_id": "string (required)",
    "listing_id": "string (required)",
    "status": "string (required)",
    "preview": {
      "title": "string (required)",
      "imageUrl": "string (optional)"
    }
  }
}
```

#### 3. negotiation_proposal
```json
{
  "type": "negotiation_proposal",
  "data": {
    "session_id": "string (required)",
    "proposal_sequence": "number (required)",
    "price": "number (required)",
    "resource_type": "string (optional)",
    "resource_id": "string (optional)",
    "note": "string (optional)"
  }
}
```

#### 4. negotiation_result
```json
{
  "type": "negotiation_result",
  "data": {
    "negotiation_id": "string (required)",
    "listing_id": "string (required)",
    "status": "string (required)",
    "preview": {
      "title": "string (required)",
      "imageUrl": "string (optional)"
    }
  }
}
```

#### 5. shipping_quote
```json
{
  "type": "shipping_quote",
  "data": {
    "offer_id": "string (required)",
    "linked_item_id": "string (required)",
    "linked_item_type": "string (required)"
  }
}
```

### True Attachments (local payload)

#### 6. location
```json
{
  "type": "location",
  "data": {
    "latitude": "number (required)",
    "longitude": "number (required)",
    "placeName": "string (optional)",
    "address": "string (optional)"
  }
}
```

## Validation Rules

1. **Structure**: Must have `type` (string) and `data` (object) fields
2. **Type**: Must be one of the valid types listed above
3. **Data**: Must match the schema for the specified type
4. **Unknown fields**: REJECTED
5. **Unknown types**: REJECTED

## Migration

Legacy wire types (`listing`, `auction`, `post`, `request`) are not canonical
attachment types in V2 and are rejected by validator authority.


