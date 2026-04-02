package database

import (
	"context"

	"github.com/google/uuid"
)

// AdminURLWithOwner extends Url with owner email for admin views
type AdminURLWithOwner struct {
	Url
	OwnerEmail string `json:"owner_email"`
}

const adminGetAllURLs = `-- name: AdminGetAllURLs :many
SELECT u.id, u.user_id, u.short_id, u.original_url, u.expiry, u.is_active, u.created_at, u.updated_at, u.password_hash, u.is_locked, u.password_attempts, u.last_password_attempt, u.team_id, COALESCE(usr.email, '') as owner_email
FROM urls u
LEFT JOIN users usr ON u.user_id = usr.id
ORDER BY u.created_at DESC
`

func (q *Queries) AdminGetAllURLs(ctx context.Context) ([]AdminURLWithOwner, error) {
	rows, err := q.db.QueryContext(ctx, adminGetAllURLs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []AdminURLWithOwner
	for rows.Next() {
		var i AdminURLWithOwner
		if err := rows.Scan(
			&i.ID,
			&i.UserID,
			&i.ShortID,
			&i.OriginalUrl,
			&i.Expiry,
			&i.IsActive,
			&i.CreatedAt,
			&i.UpdatedAt,
			&i.PasswordHash,
			&i.IsLocked,
			&i.PasswordAttempts,
			&i.LastPasswordAttempt,
			&i.TeamID,
			&i.OwnerEmail,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const adminHardDeleteURL = `-- name: AdminHardDeleteURL :exec
DELETE FROM urls WHERE id = $1
`

func (q *Queries) AdminHardDeleteURL(ctx context.Context, id uuid.UUID) error {
	_, err := q.db.ExecContext(ctx, adminHardDeleteURL, id)
	return err
}

const adminDeleteURLClicks = `-- name: AdminDeleteURLClicks :exec
DELETE FROM url_clicks WHERE url_id = $1
`

func (q *Queries) AdminDeleteURLClicks(ctx context.Context, urlID uuid.UUID) error {
	_, err := q.db.ExecContext(ctx, adminDeleteURLClicks, urlID)
	return err
}

const adminGetURLByID = `-- name: AdminGetURLByID :one
SELECT id, user_id, short_id, original_url, expiry, is_active, created_at, updated_at, password_hash, is_locked, password_attempts, last_password_attempt, team_id FROM urls WHERE id = $1
`

func (q *Queries) AdminGetURLByID(ctx context.Context, id uuid.UUID) (Url, error) {
	row := q.db.QueryRowContext(ctx, adminGetURLByID, id)
	var i Url
	err := row.Scan(
		&i.ID,
		&i.UserID,
		&i.ShortID,
		&i.OriginalUrl,
		&i.Expiry,
		&i.IsActive,
		&i.CreatedAt,
		&i.UpdatedAt,
		&i.PasswordHash,
		&i.IsLocked,
		&i.PasswordAttempts,
		&i.LastPasswordAttempt,
		&i.TeamID,
	)
	return i, err
}
