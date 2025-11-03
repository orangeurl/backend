package teams

import (
	"database/sql"
	"log"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/xenonnn4w/orangeurl/internal/database"
	"github.com/xenonnn4w/orangeurl/internal/middleware"
)

const MaxTeamMembers = 5

// CreateTeam creates a new team (Premium only, one per user)
func CreateTeam(c *fiber.Ctx) error {
	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	// Check if user is premium
	var tier string
	err = database.DB.QueryRowContext(c.Context(),
		"SELECT tier FROM subscriptions WHERE user_id = $1 AND status = 'active' ORDER BY created_at DESC LIMIT 1",
		user.ID).Scan(&tier)

	if err != nil || tier != "premium" {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Team collaboration is only available for Premium users",
		})
	}

	// Check if user already has a team
	queries := database.GetQueries()
	_, err = queries.GetTeamByOwnerID(c.Context(), user.ID)
	if err == nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "You already have a team. Each user can only create one team.",
		})
	}

	// Parse request
	type CreateTeamRequest struct {
		Name string `json:"name"`
	}
	var req CreateTeamRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request"})
	}

	// Validate team name
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Team name is required"})
	}
	if len(req.Name) > 100 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Team name must be less than 100 characters"})
	}

	// Create team
	team, err := queries.CreateTeam(c.Context(), database.CreateTeamParams{
		Name:    req.Name,
		OwnerID: user.ID,
	})
	if err != nil {
		log.Printf("[CreateTeam] Error creating team: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create team"})
	}

	// Add owner as team member
	_, err = queries.AddTeamMember(c.Context(), database.AddTeamMemberParams{
		TeamID: team.ID,
		UserID: user.ID,
	})
	if err != nil {
		log.Printf("[CreateTeam] Error adding owner to team: %v", err)
		// Rollback team creation
		queries.DeleteTeam(c.Context(), team.ID)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create team"})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"team": fiber.Map{
			"id":         team.ID,
			"name":       team.Name,
			"owner_id":   team.OwnerID,
			"created_at": team.CreatedAt,
		},
	})
}

// GetMyTeam returns the user's team (either owned or member of)
func GetMyTeam(c *fiber.Ctx) error {
	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	queries := database.GetQueries()

	// First check if user owns a team
	team, err := queries.GetTeamByOwnerID(c.Context(), user.ID)
	if err != nil && err != sql.ErrNoRows {
		log.Printf("[GetMyTeam] Error fetching owned team: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch team"})
	}

	// If not owner, check if user is a member of any team
	if err == sql.ErrNoRows {
		team, err = queries.GetTeamByMemberUserID(c.Context(), user.ID)
		if err != nil {
			if err == sql.ErrNoRows {
				return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "You are not part of any team"})
			}
			log.Printf("[GetMyTeam] Error fetching member team: %v", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch team"})
		}
	}

	// Get team members
	members, err := queries.GetTeamMembers(c.Context(), team.ID)
	if err != nil {
		log.Printf("[GetMyTeam] Error fetching team members: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch team members"})
	}

	// Check if current user is owner
	isOwner := team.OwnerID == user.ID

	return c.JSON(fiber.Map{
		"team": fiber.Map{
			"id":         team.ID,
			"name":       team.Name,
			"owner_id":   team.OwnerID,
			"is_owner":   isOwner,
			"created_at": team.CreatedAt,
		},
		"members": members,
	})
}

// AddTeamMember adds a member to the team
func AddTeamMemberHandler(c *fiber.Ctx) error {
	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	queries := database.GetQueries()

	// Get user's team (must be owner)
	team, err := queries.GetTeamByOwnerID(c.Context(), user.ID)
	if err != nil {
		if err == sql.ErrNoRows {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "You don't have a team or you're not the owner"})
		}
		log.Printf("[AddTeamMember] Error fetching team: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch team"})
	}

	// Parse request
	type AddMemberRequest struct {
		Email string `json:"email"`
	}
	var req AddMemberRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request"})
	}

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	if req.Email == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Email is required"})
	}

	// Check member count (owner + members <= 5)
	memberCount, err := queries.GetTeamMemberCount(c.Context(), team.ID)
	if err != nil {
		log.Printf("[AddTeamMember] Error counting members: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to check member count"})
	}

	if memberCount >= MaxTeamMembers {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Team is full. Maximum 5 members allowed.",
		})
	}

	// Find user by email
	memberUser, err := queries.GetUserByEmail(c.Context(), req.Email)
	if err != nil {
		if err == sql.ErrNoRows {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "User with this email is not registered on OrangeURL",
			})
		}
		log.Printf("[AddTeamMember] Error finding user: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to find user"})
	}

	// Check if user is already in team
	isInTeam, err := queries.IsUserInTeam(c.Context(), database.IsUserInTeamParams{
		TeamID: team.ID,
		UserID: memberUser.ID,
	})
	if err != nil {
		log.Printf("[AddTeamMember] Error checking membership: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to check membership"})
	}

	if isInTeam {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "User is already a team member"})
	}

	// Add member
	_, err = queries.AddTeamMember(c.Context(), database.AddTeamMemberParams{
		TeamID: team.ID,
		UserID: memberUser.ID,
	})
	if err != nil {
		log.Printf("[AddTeamMember] Error adding member: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to add team member"})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"message": "Team member added successfully",
		"member": fiber.Map{
			"email":    memberUser.Email,
			"user_id":  memberUser.ID,
			"clerk_id": memberUser.ClerkID,
		},
	})
}

// RemoveTeamMember removes a member from the team
func RemoveTeamMemberHandler(c *fiber.Ctx) error {
	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	queries := database.GetQueries()

	// Get user's team (must be owner)
	team, err := queries.GetTeamByOwnerID(c.Context(), user.ID)
	if err != nil {
		if err == sql.ErrNoRows {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "You don't have a team or you're not the owner"})
		}
		log.Printf("[RemoveTeamMember] Error fetching team: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch team"})
	}

	// Get member ID from params
	memberIDStr := c.Params("memberID")
	if memberIDStr == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Member ID is required"})
	}

	// Parse UUID
	memberID, err := uuid.Parse(memberIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid member ID"})
	}

	// Cannot remove yourself (owner)
	if memberID == user.ID {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot remove yourself from the team"})
	}

	// Remove member
	err = queries.RemoveTeamMember(c.Context(), database.RemoveTeamMemberParams{
		TeamID: team.ID,
		UserID: memberID,
	})
	if err != nil {
		log.Printf("[RemoveTeamMember] Error removing member: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to remove team member"})
	}

	return c.JSON(fiber.Map{"message": "Team member removed successfully"})
}

// DeleteTeam deletes the team (owner only)
func DeleteTeam(c *fiber.Ctx) error {
	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	queries := database.GetQueries()

	// Get user's team (must be owner)
	team, err := queries.GetTeamByOwnerID(c.Context(), user.ID)
	if err != nil {
		if err == sql.ErrNoRows {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "You don't have a team"})
		}
		log.Printf("[DeleteTeam] Error fetching team: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch team"})
	}

	// Delete team (cascade will handle team_members)
	err = queries.DeleteTeam(c.Context(), team.ID)
	if err != nil {
		log.Printf("[DeleteTeam] Error deleting team: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to delete team"})
	}

	return c.JSON(fiber.Map{"message": "Team deleted successfully"})
}
