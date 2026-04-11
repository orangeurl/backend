package database

import (
	"context"

	"github.com/google/uuid"
)

// -- Blocked Domains --

type BlockDomainParams struct {
	Domain            string        `json:"domain"`
	IncludeSubdomains bool          `json:"include_subdomains"`
	BlockReason       string        `json:"block_reason"`
	BlockedBy         uuid.NullUUID `json:"blocked_by"`
}

const addBlockedDomain = `
INSERT INTO blocked_domains (domain, include_subdomains, block_reason, blocked_by)
VALUES ($1, $2, $3, $4)
ON CONFLICT (domain) DO UPDATE SET
    include_subdomains = EXCLUDED.include_subdomains,
    block_reason = EXCLUDED.block_reason,
    blocked_by = EXCLUDED.blocked_by,
    created_at = NOW()
RETURNING id, domain, include_subdomains, block_reason, blocked_by, created_at
`

func (q *Queries) AddBlockedDomain(ctx context.Context, arg BlockDomainParams) (BlockedDomain, error) {
	row := q.db.QueryRowContext(ctx, addBlockedDomain,
		arg.Domain,
		arg.IncludeSubdomains,
		arg.BlockReason,
		arg.BlockedBy,
	)
	var i BlockedDomain
	err := row.Scan(
		&i.ID,
		&i.Domain,
		&i.IncludeSubdomains,
		&i.BlockReason,
		&i.BlockedBy,
		&i.CreatedAt,
	)
	return i, err
}

const removeBlockedDomain = `DELETE FROM blocked_domains WHERE id = $1`

func (q *Queries) RemoveBlockedDomain(ctx context.Context, id uuid.UUID) error {
	_, err := q.db.ExecContext(ctx, removeBlockedDomain, id)
	return err
}

const listBlockedDomains = `
SELECT id, domain, include_subdomains, block_reason, blocked_by, created_at
FROM blocked_domains ORDER BY created_at DESC
`

func (q *Queries) ListBlockedDomains(ctx context.Context) ([]BlockedDomain, error) {
	rows, err := q.db.QueryContext(ctx, listBlockedDomains)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []BlockedDomain
	for rows.Next() {
		var i BlockedDomain
		if err := rows.Scan(
			&i.ID,
			&i.Domain,
			&i.IncludeSubdomains,
			&i.BlockReason,
			&i.BlockedBy,
			&i.CreatedAt,
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

// -- User Banning --

const banUser = `UPDATE users SET is_banned = TRUE, updated_at = NOW() WHERE id = $1`

func (q *Queries) BanUser(ctx context.Context, id uuid.UUID) error {
	_, err := q.db.ExecContext(ctx, banUser, id)
	return err
}

const unbanUser = `UPDATE users SET is_banned = FALSE, updated_at = NOW() WHERE id = $1`

func (q *Queries) UnbanUser(ctx context.Context, id uuid.UUID) error {
	_, err := q.db.ExecContext(ctx, unbanUser, id)
	return err
}

const adminListAllUsers = `
SELECT id, clerk_id, email, first_name, last_name, avatar_url, subscription_tier, created_at, updated_at, is_admin, is_banned
FROM users ORDER BY created_at DESC
`

func (q *Queries) AdminListAllUsers(ctx context.Context) ([]User, error) {
	rows, err := q.db.QueryContext(ctx, adminListAllUsers)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []User
	for rows.Next() {
		var i User
		if err := rows.Scan(
			&i.ID,
			&i.ClerkID,
			&i.Email,
			&i.FirstName,
			&i.LastName,
			&i.AvatarUrl,
			&i.SubscriptionTier,
			&i.CreatedAt,
			&i.UpdatedAt,
			&i.IsAdmin,
			&i.IsBanned,
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
