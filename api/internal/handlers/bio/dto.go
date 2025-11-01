package bio

import (
	"encoding/json"

	"github.com/xenonnn4w/orangeurl/internal/database"
)

// BioPageResponse is the DTO for bio page responses
type BioPageResponse struct {
	ID              string          `json:"id"`
	UserID          string          `json:"user_id"`
	Username        string          `json:"username"`
	DisplayName     string          `json:"display_name"`
	Bio             *string         `json:"bio"`
	AvatarURL       *string         `json:"avatar_url"`
	FaviconURL      *string         `json:"favicon_url"`
	Theme           string          `json:"theme"`
	CustomColors    json.RawMessage `json:"custom_colors"`
	BackgroundType  string          `json:"background_type"`
	BackgroundValue *string         `json:"background_value"`
	ButtonStyle     string          `json:"button_style"`
	FontFamily      string          `json:"font_family"`
	SocialLinks     json.RawMessage `json:"social_links"`
	Links           json.RawMessage `json:"links"`
	SEOTitle        *string         `json:"seo_title"`
	SEODescription  *string         `json:"seo_description"`
	OGImageURL      *string         `json:"og_image_url"`
	IsPublished     bool            `json:"is_published"`
	Subdomain       *string         `json:"subdomain"`
	TotalViews      int32           `json:"total_views"`
	TotalClicks     int32           `json:"total_clicks"`
	CreatedAt       string          `json:"created_at"`
	UpdatedAt       string          `json:"updated_at"`
	LastPublishedAt *string         `json:"last_published_at"`
}

// ConvertBioPageToResponse converts database.BioPage to BioPageResponse
func ConvertBioPageToResponse(bp database.BioPage) BioPageResponse {
	resp := BioPageResponse{
		ID:          bp.ID.String(),
		UserID:      bp.UserID.String(),
		Username:    bp.Username,
		DisplayName: bp.DisplayName,
		TotalViews:  bp.TotalViews.Int32,
		TotalClicks: bp.TotalClicks.Int32,
	}

	// Handle nullable strings
	if bp.Bio.Valid {
		resp.Bio = &bp.Bio.String
	}
	if bp.AvatarUrl.Valid {
		resp.AvatarURL = &bp.AvatarUrl.String
	}
	if bp.FaviconUrl.Valid {
		resp.FaviconURL = &bp.FaviconUrl.String
	}
	if bp.Theme.Valid {
		resp.Theme = bp.Theme.String
	}
	if bp.BackgroundType.Valid {
		resp.BackgroundType = bp.BackgroundType.String
	}
	if bp.BackgroundValue.Valid {
		resp.BackgroundValue = &bp.BackgroundValue.String
	}
	if bp.ButtonStyle.Valid {
		resp.ButtonStyle = bp.ButtonStyle.String
	}
	if bp.FontFamily.Valid {
		resp.FontFamily = bp.FontFamily.String
	}
	if bp.SeoTitle.Valid {
		resp.SEOTitle = &bp.SeoTitle.String
	}
	if bp.SeoDescription.Valid {
		resp.SEODescription = &bp.SeoDescription.String
	}
	if bp.OgImageUrl.Valid {
		resp.OGImageURL = &bp.OgImageUrl.String
	}
	if bp.Subdomain.Valid {
		resp.Subdomain = &bp.Subdomain.String
	}

	// Handle nullable bools
	if bp.IsPublished.Valid {
		resp.IsPublished = bp.IsPublished.Bool
	}

	// Handle JSONB fields
	if bp.CustomColors.Valid {
		resp.CustomColors = bp.CustomColors.RawMessage
	} else {
		// Default custom colors
		resp.CustomColors = json.RawMessage(`{"primary":"#ea580c","secondary":"#fb923c","background":"#ffffff","text":"#1f2937"}`)
	}

	if bp.SocialLinks.Valid {
		resp.SocialLinks = bp.SocialLinks.RawMessage
	} else {
		resp.SocialLinks = json.RawMessage(`{}`)
	}

	if bp.Links.Valid {
		resp.Links = bp.Links.RawMessage
	} else {
		resp.Links = json.RawMessage(`[]`)
	}

	// Handle timestamps
	if bp.CreatedAt.Valid {
		resp.CreatedAt = bp.CreatedAt.Time.Format("2006-01-02T15:04:05Z07:00")
	}
	if bp.UpdatedAt.Valid {
		resp.UpdatedAt = bp.UpdatedAt.Time.Format("2006-01-02T15:04:05Z07:00")
	}
	if bp.LastPublishedAt.Valid {
		publishedAt := bp.LastPublishedAt.Time.Format("2006-01-02T15:04:05Z07:00")
		resp.LastPublishedAt = &publishedAt
	}

	return resp
}
