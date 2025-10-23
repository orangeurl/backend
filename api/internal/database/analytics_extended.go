// Extended analytics functions for OrangeURL
package database

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
)

// GetURLAnalyticsWithLimitParams contains parameters for the query
type GetURLAnalyticsWithLimitParams struct {
	UrlID uuid.UUID
	Limit int32
}

// GetURLAnalyticsWithLimit returns limited analytics for a URL
func (q *Queries) GetURLAnalyticsWithLimit(ctx context.Context, arg GetURLAnalyticsWithLimitParams) ([]UrlClick, error) {
	query := `SELECT id, url_id, ip_address, user_agent, referer, country, city, device_type, browser, os, is_bot, clicked_at 
	          FROM url_clicks WHERE url_id = $1 ORDER BY clicked_at DESC LIMIT $2`
	
	rows, err := q.db.QueryContext(ctx, query, arg.UrlID, arg.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []UrlClick
	for rows.Next() {
		var i UrlClick
		if err := rows.Scan(
			&i.ID,
			&i.UrlID,
			&i.IpAddress,
			&i.UserAgent,
			&i.Referer,
			&i.Country,
			&i.City,
			&i.DeviceType,
			&i.Browser,
			&i.Os,
			&i.IsBot,
			&i.ClickedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	return items, rows.Err()
}

// GetUserAnalyticsWithDateRangeParams contains parameters for the query
type GetUserAnalyticsWithDateRangeParams struct {
	UserID    uuid.UUID
	ClickedAt time.Time
	ClickedAt_2 time.Time
}

// GetUserAnalyticsWithDateRange returns analytics within a date range
func (q *Queries) GetUserAnalyticsWithDateRange(ctx context.Context, arg GetUserAnalyticsWithDateRangeParams) ([]UrlClick, error) {
	query := `SELECT uc.id, uc.url_id, uc.ip_address, uc.user_agent, uc.referer, uc.country, uc.city, uc.device_type, uc.browser, uc.os, uc.is_bot, uc.clicked_at 
	          FROM url_clicks uc JOIN urls u ON uc.url_id = u.id 
	          WHERE u.user_id = $1 AND uc.clicked_at >= $2 AND uc.clicked_at <= $3 ORDER BY uc.clicked_at DESC`
	
	rows, err := q.db.QueryContext(ctx, query, arg.UserID, arg.ClickedAt, arg.ClickedAt_2)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []UrlClick
	for rows.Next() {
		var i UrlClick
		if err := rows.Scan(
			&i.ID,
			&i.UrlID,
			&i.IpAddress,
			&i.UserAgent,
			&i.Referer,
			&i.Country,
			&i.City,
			&i.DeviceType,
			&i.Browser,
			&i.Os,
			&i.IsBot,
			&i.ClickedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	return items, rows.Err()
}

// GetURLClickCount returns the total click count for a URL
func (q *Queries) GetURLClickCount(ctx context.Context, urlID uuid.UUID) (int64, error) {
	query := `SELECT COUNT(*) as click_count FROM url_clicks WHERE url_id = $1`
	var count int64
	err := q.db.QueryRowContext(ctx, query, urlID).Scan(&count)
	return count, err
}

// GetUserTotalClicks returns total clicks for a user
func (q *Queries) GetUserTotalClicks(ctx context.Context, userID uuid.UUID) (int64, error) {
	query := `SELECT COUNT(*) as total_clicks FROM url_clicks uc JOIN urls u ON uc.url_id = u.id WHERE u.user_id = $1`
	var count int64
	err := q.db.QueryRowContext(ctx, query, userID).Scan(&count)
	return count, err
}

// CountryClickRow represents clicks by country
type CountryClickRow struct {
	Country    sql.NullString
	ClickCount int64
}

// GetURLClicksByCountry returns clicks grouped by country
func (q *Queries) GetURLClicksByCountry(ctx context.Context, urlID uuid.UUID) ([]CountryClickRow, error) {
	query := `SELECT country, COUNT(*) as click_count FROM url_clicks 
	          WHERE url_id = $1 AND country IS NOT NULL GROUP BY country ORDER BY click_count DESC`
	
	rows, err := q.db.QueryContext(ctx, query, urlID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []CountryClickRow
	for rows.Next() {
		var i CountryClickRow
		if err := rows.Scan(&i.Country, &i.ClickCount); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	return items, rows.Err()
}

// GetUserClicksByCountry returns user clicks grouped by country
func (q *Queries) GetUserClicksByCountry(ctx context.Context, userID uuid.UUID) ([]CountryClickRow, error) {
	query := `SELECT uc.country, COUNT(*) as click_count FROM url_clicks uc JOIN urls u ON uc.url_id = u.id 
	          WHERE u.user_id = $1 AND uc.country IS NOT NULL GROUP BY uc.country ORDER BY click_count DESC`
	
	rows, err := q.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []CountryClickRow
	for rows.Next() {
		var i CountryClickRow
		if err := rows.Scan(&i.Country, &i.ClickCount); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	return items, rows.Err()
}

// DeviceClickRow represents clicks by device
type DeviceClickRow struct {
	DeviceType sql.NullString
	ClickCount int64
}

// GetURLClicksByDevice returns clicks grouped by device
func (q *Queries) GetURLClicksByDevice(ctx context.Context, urlID uuid.UUID) ([]DeviceClickRow, error) {
	query := `SELECT device_type, COUNT(*) as click_count FROM url_clicks 
	          WHERE url_id = $1 AND device_type IS NOT NULL GROUP BY device_type ORDER BY click_count DESC`
	
	rows, err := q.db.QueryContext(ctx, query, urlID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []DeviceClickRow
	for rows.Next() {
		var i DeviceClickRow
		if err := rows.Scan(&i.DeviceType, &i.ClickCount); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	return items, rows.Err()
}

// GetUserClicksByDevice returns user clicks grouped by device
func (q *Queries) GetUserClicksByDevice(ctx context.Context, userID uuid.UUID) ([]DeviceClickRow, error) {
	query := `SELECT uc.device_type, COUNT(*) as click_count FROM url_clicks uc JOIN urls u ON uc.url_id = u.id 
	          WHERE u.user_id = $1 AND uc.device_type IS NOT NULL GROUP BY uc.device_type ORDER BY click_count DESC`
	
	rows, err := q.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []DeviceClickRow
	for rows.Next() {
		var i DeviceClickRow
		if err := rows.Scan(&i.DeviceType, &i.ClickCount); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	return items, rows.Err()
}

// BrowserClickRow represents clicks by browser
type BrowserClickRow struct {
	Browser    sql.NullString
	ClickCount int64
}

// GetURLClicksByBrowser returns clicks grouped by browser
func (q *Queries) GetURLClicksByBrowser(ctx context.Context, urlID uuid.UUID) ([]BrowserClickRow, error) {
	query := `SELECT browser, COUNT(*) as click_count FROM url_clicks 
	          WHERE url_id = $1 AND browser IS NOT NULL GROUP BY browser ORDER BY click_count DESC`
	
	rows, err := q.db.QueryContext(ctx, query, urlID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []BrowserClickRow
	for rows.Next() {
		var i BrowserClickRow
		if err := rows.Scan(&i.Browser, &i.ClickCount); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	return items, rows.Err()
}

// GetUserClicksByBrowser returns user clicks grouped by browser
func (q *Queries) GetUserClicksByBrowser(ctx context.Context, userID uuid.UUID) ([]BrowserClickRow, error) {
	query := `SELECT uc.browser, COUNT(*) as click_count FROM url_clicks uc JOIN urls u ON uc.url_id = u.id 
	          WHERE u.user_id = $1 AND uc.browser IS NOT NULL GROUP BY uc.browser ORDER BY click_count DESC`
	
	rows, err := q.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []BrowserClickRow
	for rows.Next() {
		var i BrowserClickRow
		if err := rows.Scan(&i.Browser, &i.ClickCount); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	return items, rows.Err()
}

// OSClickRow represents clicks by OS
type OSClickRow struct {
	Os         sql.NullString
	ClickCount int64
}

// GetURLClicksByOS returns clicks grouped by OS
func (q *Queries) GetURLClicksByOS(ctx context.Context, urlID uuid.UUID) ([]OSClickRow, error) {
	query := `SELECT os, COUNT(*) as click_count FROM url_clicks 
	          WHERE url_id = $1 AND os IS NOT NULL GROUP BY os ORDER BY click_count DESC`
	
	rows, err := q.db.QueryContext(ctx, query, urlID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []OSClickRow
	for rows.Next() {
		var i OSClickRow
		if err := rows.Scan(&i.Os, &i.ClickCount); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	return items, rows.Err()
}

// GetUserClicksByOS returns user clicks grouped by OS
func (q *Queries) GetUserClicksByOS(ctx context.Context, userID uuid.UUID) ([]OSClickRow, error) {
	query := `SELECT uc.os, COUNT(*) as click_count FROM url_clicks uc JOIN urls u ON uc.url_id = u.id 
	          WHERE u.user_id = $1 AND uc.os IS NOT NULL GROUP BY uc.os ORDER BY click_count DESC`
	
	rows, err := q.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []OSClickRow
	for rows.Next() {
		var i OSClickRow
		if err := rows.Scan(&i.Os, &i.ClickCount); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	return items, rows.Err()
}

// TimeSeriesRow represents clicks over time
type TimeSeriesRow struct {
	Date       sql.NullTime
	ClickCount int64
}

// GetURLClicksOverTimeParams contains parameters
type GetURLClicksOverTimeParams struct {
	UrlID     uuid.UUID
	ClickedAt time.Time
}

// GetURLClicksOverTime returns clicks over time for a URL
func (q *Queries) GetURLClicksOverTime(ctx context.Context, arg GetURLClicksOverTimeParams) ([]TimeSeriesRow, error) {
	query := `SELECT DATE(clicked_at) as date, COUNT(*) as click_count FROM url_clicks 
	          WHERE url_id = $1 AND clicked_at >= $2 GROUP BY DATE(clicked_at) ORDER BY date DESC`
	
	rows, err := q.db.QueryContext(ctx, query, arg.UrlID, arg.ClickedAt)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []TimeSeriesRow
	for rows.Next() {
		var i TimeSeriesRow
		if err := rows.Scan(&i.Date, &i.ClickCount); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	return items, rows.Err()
}

// GetUserClicksOverTimeParams contains parameters
type GetUserClicksOverTimeParams struct {
	UserID    uuid.UUID
	ClickedAt time.Time
}

// GetUserClicksOverTime returns clicks over time for a user
func (q *Queries) GetUserClicksOverTime(ctx context.Context, arg GetUserClicksOverTimeParams) ([]TimeSeriesRow, error) {
	query := `SELECT DATE(uc.clicked_at) as date, COUNT(*) as click_count FROM url_clicks uc JOIN urls u ON uc.url_id = u.id 
	          WHERE u.user_id = $1 AND uc.clicked_at >= $2 GROUP BY DATE(uc.clicked_at) ORDER BY date DESC`
	
	rows, err := q.db.QueryContext(ctx, query, arg.UserID, arg.ClickedAt)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []TimeSeriesRow
	for rows.Next() {
		var i TimeSeriesRow
		if err := rows.Scan(&i.Date, &i.ClickCount); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	return items, rows.Err()
}

// TopURLRow represents top URLs
type TopURLRow struct {
	ID          uuid.UUID
	ShortID     string
	OriginalUrl string
	ClickCount  sql.NullInt64
}

// GetUserTopURLsParams contains parameters
type GetUserTopURLsParams struct {
	UserID uuid.UUID
	Limit  int32
}

// GetUserTopURLs returns top URLs for a user
func (q *Queries) GetUserTopURLs(ctx context.Context, arg GetUserTopURLsParams) ([]TopURLRow, error) {
	query := `SELECT u.id, u.short_id, u.original_url, COUNT(uc.id) as click_count FROM urls u 
	          LEFT JOIN url_clicks uc ON u.id = uc.url_id 
	          WHERE u.user_id = $1 AND u.is_active = TRUE 
	          GROUP BY u.id, u.short_id, u.original_url ORDER BY click_count DESC LIMIT $2`
	
	rows, err := q.db.QueryContext(ctx, query, arg.UserID, arg.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []TopURLRow
	for rows.Next() {
		var i TopURLRow
		if err := rows.Scan(&i.ID, &i.ShortID, &i.OriginalUrl, &i.ClickCount); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	return items, rows.Err()
}

// ReferrerRow represents referrer data
type ReferrerRow struct {
	Referer    sql.NullString
	ClickCount int64
}

// GetURLReferrersParams contains parameters
type GetURLReferrersParams struct {
	UrlID uuid.UUID
	Limit int32
}

// GetURLReferrers returns top referrers for a URL
func (q *Queries) GetURLReferrers(ctx context.Context, arg GetURLReferrersParams) ([]ReferrerRow, error) {
	query := `SELECT referer, COUNT(*) as click_count FROM url_clicks 
	          WHERE url_id = $1 AND referer IS NOT NULL AND referer != '' 
	          GROUP BY referer ORDER BY click_count DESC LIMIT $2`
	
	rows, err := q.db.QueryContext(ctx, query, arg.UrlID, arg.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []ReferrerRow
	for rows.Next() {
		var i ReferrerRow
		if err := rows.Scan(&i.Referer, &i.ClickCount); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	return items, rows.Err()
}

// GetUserBotClicksPercentage returns percentage of bot clicks
func (q *Queries) GetUserBotClicksPercentage(ctx context.Context, userID uuid.UUID) (sql.NullFloat64, error) {
	query := `SELECT COUNT(CASE WHEN uc.is_bot = TRUE THEN 1 END)::FLOAT / NULLIF(COUNT(*), 0) * 100 as bot_percentage 
	          FROM url_clicks uc JOIN urls u ON uc.url_id = u.id WHERE u.user_id = $1`
	
	var percentage sql.NullFloat64
	err := q.db.QueryRowContext(ctx, query, userID).Scan(&percentage)
	return percentage, err
}

// DeleteOldClicksByDateParams contains parameters
type DeleteOldClicksByDateParams struct {
	UserID    uuid.UUID
	ClickedAt time.Time
}

// DeleteOldClicksByDate deletes old clicks based on retention policy
func (q *Queries) DeleteOldClicksByDate(ctx context.Context, arg DeleteOldClicksByDateParams) error {
	query := `DELETE FROM url_clicks WHERE url_id IN (SELECT id FROM urls WHERE user_id = $1) AND clicked_at < $2`
	_, err := q.db.ExecContext(ctx, query, arg.UserID, arg.ClickedAt)
	return err
}

