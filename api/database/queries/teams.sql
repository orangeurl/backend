-- name: CreateTeam :one
INSERT INTO teams (name, owner_id)
VALUES ($1, $2)
RETURNING *;

-- name: GetTeamByID :one
SELECT * FROM teams
WHERE id = $1 LIMIT 1;

-- name: GetTeamByOwnerID :one
SELECT * FROM teams
WHERE owner_id = $1 LIMIT 1;

-- name: UpdateTeamName :one
UPDATE teams
SET name = $1
WHERE id = $2
RETURNING *;

-- name: DeleteTeam :exec
DELETE FROM teams
WHERE id = $1;

-- name: AddTeamMember :one
INSERT INTO team_members (team_id, user_id)
VALUES ($1, $2)
RETURNING *;

-- name: RemoveTeamMember :exec
DELETE FROM team_members
WHERE team_id = $1 AND user_id = $2;

-- name: GetTeamMembers :many
SELECT
    tm.id,
    tm.team_id,
    tm.user_id,
    tm.joined_at,
    u.email,
    u.clerk_id
FROM team_members tm
JOIN users u ON tm.user_id = u.id
WHERE tm.team_id = $1
ORDER BY tm.joined_at ASC;

-- name: GetTeamMemberCount :one
SELECT COUNT(*) FROM team_members
WHERE team_id = $1;

-- name: IsUserInTeam :one
SELECT EXISTS(
    SELECT 1 FROM team_members
    WHERE team_id = $1 AND user_id = $2
);

-- name: GetUserTeams :many
SELECT t.* FROM teams t
JOIN team_members tm ON t.id = tm.team_id
WHERE tm.user_id = $1
ORDER BY t.created_at DESC;

-- name: GetTeamByMemberUserID :one
SELECT t.* FROM teams t
JOIN team_members tm ON t.id = tm.team_id
WHERE tm.user_id = $1
LIMIT 1;

-- name: GetUserOwnedTeam :one
SELECT * FROM teams
WHERE owner_id = $1
LIMIT 1;

-- name: IsTeamOwner :one
SELECT EXISTS(
    SELECT 1 FROM teams
    WHERE id = $1 AND owner_id = $2
);

-- name: GetTeamURLs :many
SELECT * FROM urls
WHERE team_id = $1
ORDER BY created_at DESC;

-- name: AssignURLToTeam :exec
UPDATE urls
SET team_id = $1
WHERE id = $2;

-- name: UnassignURLFromTeam :exec
UPDATE urls
SET team_id = NULL
WHERE id = $1;

-- name: CanUserAccessURL :one
SELECT EXISTS(
    SELECT 1 FROM urls u
    LEFT JOIN team_members tm ON u.team_id = tm.team_id
    WHERE u.id = $1
    AND (u.user_id = $2 OR tm.user_id = $2)
);
