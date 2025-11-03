package database

import (
	"database/sql"

	"github.com/sqlc-dev/pqtype"
)

// NewNullString creates a sql.NullString
func NewNullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: s, Valid: true}
}

// NewNullInt32 creates a sql.NullInt32
func NewNullInt32(i int32) sql.NullInt32 {
	return sql.NullInt32{Int32: i, Valid: true}
}

// NewNullInt64 creates a sql.NullInt64
func NewNullInt64(i int64) sql.NullInt64 {
	return sql.NullInt64{Int64: i, Valid: true}
}

// NewNullBool creates a sql.NullBool
func NewNullBool(b bool) sql.NullBool {
	return sql.NullBool{Bool: b, Valid: true}
}

// NewNullRawMessage creates a pqtype.NullRawMessage
func NewNullRawMessage(data []byte) pqtype.NullRawMessage {
	if data == nil {
		return pqtype.NullRawMessage{Valid: false}
	}
	return pqtype.NullRawMessage{RawMessage: data, Valid: true}
}
