package registry

import (
	"centralisd/src/orchestrator/db"
	"gorm.io/gorm"
)

type AllowedClusters struct {
	db *gorm.DB
}

func NewAllowedClusters(dbConn *gorm.DB) *AllowedClusters {
	return &AllowedClusters{db: dbConn}
}

func (a *AllowedClusters) AutoMigrate() error {
	if a == nil || a.db == nil {
		return nil
	}
	return a.db.AutoMigrate(&db.AllowedCluster{})
}

func (a *AllowedClusters) EnsureSeed(clusters []string) error {
	if a == nil || a.db == nil {
		return nil
	}
	for _, c := range clusters {
		if c == "" {
			continue
		}
		// INSERT IGNORE equivalent
		_ = a.db.FirstOrCreate(&db.AllowedCluster{}, db.AllowedCluster{ClusterID: c}).Error
	}
	return nil
}

func (a *AllowedClusters) List() ([]string, error) {
	if a == nil || a.db == nil {
		return nil, nil
	}
	var rows []db.AllowedCluster
	if err := a.db.Order("cluster_id asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.ClusterID)
	}
	return out, nil
}

func (a *AllowedClusters) IsAllowed(clusterID string) (bool, error) {
	if a == nil || a.db == nil {
		return true, nil
	}
	// If no clusters are configured/persisted yet, allow everything.
	var n int64
	if err := a.db.Model(&db.AllowedCluster{}).Count(&n).Error; err == nil {
		if n == 0 {
			return true, nil
		}
	}
	if clusterID == "" {
		return false, nil
	}
	var row db.AllowedCluster
	err := a.db.Where("cluster_id = ?", clusterID).First(&row).Error
	if err == nil {
		return true, nil
	}
	if err == gorm.ErrRecordNotFound {
		return false, nil
	}
	return false, err
}
