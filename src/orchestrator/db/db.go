package db

import (
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func Open(path string) (*gorm.DB, error) {
	if path == "" {
		path = "./centralis.db"
	}
	return gorm.Open(sqlite.Open(path), &gorm.Config{})
}
