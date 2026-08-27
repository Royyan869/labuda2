package db

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
)

// ToStringPtr converts sql.NullString to *string
func ToStringPtr(ns sql.NullString) *string {
	if !ns.Valid {
		return nil
	}
	return &ns.String
}

// ToTimePtr converts sql.NullTime to *time.Time
func ToTimePtr(nt sql.NullTime) *time.Time {
	if !nt.Valid {
		return nil
	}
	return &nt.Time
}

// ToBoolPtr converts sql.NullBool to *bool
func ToBoolPtr(nb sql.NullBool) *bool {
	if !nb.Valid {
		return nil
	}
	return &nb.Bool
}

// ToInt64Ptr converts sql.NullInt64 to *int64
func ToInt64Ptr(ni sql.NullInt64) *int64 {
	if !ni.Valid {
		return nil
	}
	return &ni.Int64
}

// ToFloat64Ptr converts sql.NullFloat64 to *float64
func ToFloat64Ptr(nf sql.NullFloat64) *float64 {
	if !nf.Valid {
		return nil
	}
	return &nf.Float64
}

// ToNullString converts *string to sql.NullString
func ToNullString(s *string) sql.NullString {
	if s == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *s, Valid: true}
}

// ToNullTime converts *time.Time to sql.NullTime
func ToNullTime(t *time.Time) sql.NullTime {
	if t == nil {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: *t, Valid: true}
}

// ToNullBool converts *bool to sql.NullBool
func ToNullBool(b *bool) sql.NullBool {
	if b == nil {
		return sql.NullBool{}
	}
	return sql.NullBool{Bool: *b, Valid: true}
}

// ToNullInt64 converts *int64 to sql.NullInt64
func ToNullInt64(i *int64) sql.NullInt64 {
	if i == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *i, Valid: true}
}

// ToNullFloat64 converts *float64 to sql.NullFloat64
func ToNullFloat64(f *float64) sql.NullFloat64 {
	if f == nil {
		return sql.NullFloat64{}
	}
	return sql.NullFloat64{Float64: *f, Valid: true}
}

// ToUUIDPtr converts sql.NullString to *uuid.UUID
func ToUUIDPtr(ns sql.NullString) *uuid.UUID {
	if !ns.Valid {
		return nil
	}
	parsed, err := uuid.Parse(ns.String)
	if err != nil {
		return nil
	}
	return &parsed
}

// ToNullUUID converts *uuid.UUID to sql.NullString
func ToNullUUID(id *uuid.UUID) sql.NullString {
	if id == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: id.String(), Valid: true}
}

// ToIntPtr converts sql.NullInt64 to *int
func ToIntPtr(ni sql.NullInt64) *int {
	if !ni.Valid {
		return nil
	}
	val := int(ni.Int64)
	return &val
}

// ToNullIntPtr converts *int to sql.NullInt64
func ToNullIntPtr(i *int) sql.NullInt64 {
	if i == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(*i), Valid: true}
}
