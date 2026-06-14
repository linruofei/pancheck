package model

import "time"

type InvalidLink struct {
	ID            uint      `json:"id"`
	Link          string    `json:"link"`
	Platform      Platform  `json:"platform"`
	FailureReason string    `json:"failure_reason"`
	CheckDuration *int64    `json:"check_duration"`
	IsRateLimited bool      `json:"is_rate_limited"`
	CreatedAt     time.Time `json:"created_at"`
	QueryTime     time.Time `json:"query_time"`
}
