package db

import "time"

// AllowedCluster is the only persisted schema for now.
// Everything else is in-memory state until other orchestrator features (reverse proxy, etc.) land.
type AllowedCluster struct {
	ID        uint      `gorm:"primaryKey"`
	ClusterID string    `gorm:"uniqueIndex;not null"`
	CreatedAt time.Time
	UpdatedAt time.Time
}
