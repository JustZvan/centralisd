package registry

import (
	"sort"
	"sync"
	"time"
)

type MasterInfo struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Cluster   string     `json:"cluster"`
	Advertise string     `json:"advertise"`
	PubKey    string     `json:"pubKey"`
	Nodes     []NodeInfo `json:"nodes"`

	LastSeen time.Time `json:"lastSeen"`
}

type NodeInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	IP   string `json:"ip"`
}

type Store struct {
	mu          sync.RWMutex
	mastersByID map[string]MasterInfo
	ttl         time.Duration
	allowed     *AllowedClusters
}

func NewStore(ttl time.Duration, allowed *AllowedClusters) *Store {
	if ttl <= 0 {
		ttl = 60 * time.Second
	}
	return &Store{
		mastersByID: map[string]MasterInfo{},
		ttl:         ttl,
		allowed:     allowed,
	}
}

func (s *Store) UpsertMaster(mi MasterInfo) bool {
	if mi.ID == "" {
		return false
	}
	if s.allowed != nil {
		ok, err := s.allowed.IsAllowed(mi.Cluster)
		if err != nil || !ok {
			return false
		}
	}
	mi.LastSeen = time.Now()

	s.mu.Lock()
	s.mastersByID[mi.ID] = mi
	s.mu.Unlock()
	return true
}

func (s *Store) ActiveClusters() []string {
	if s.allowed != nil {
		clusters, err := s.allowed.List()
		if err == nil {
			return clusters
		}
	}

	// Fallback: derive from active masters.
	now := time.Now()

	s.mu.RLock()
	clusters := map[string]struct{}{}
	for _, m := range s.mastersByID {
		if now.Sub(m.LastSeen) > s.ttl {
			continue
		}
		if m.Cluster == "" {
			continue
		}
		clusters[m.Cluster] = struct{}{}
	}
	s.mu.RUnlock()

	out := make([]string, 0, len(clusters))
	for c := range clusters {
		out = append(out, c)
	}
	sort.Strings(out)
	return out
}

func (s *Store) IsKnownCluster(clusterID string) bool {
	if clusterID == "" {
		return false
	}
	if s.allowed != nil {
		ok, err := s.allowed.IsAllowed(clusterID)
		if err == nil {
			return ok
		}
	}
	now := time.Now()

	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, m := range s.mastersByID {
		if now.Sub(m.LastSeen) > s.ttl {
			continue
		}
		if m.Cluster == clusterID {
			return true
		}
	}
	return false
}

func (s *Store) IsClusterOnline(clusterID string) bool {
	if clusterID == "" {
		return false
	}
	now := time.Now()

	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, m := range s.mastersByID {
		if m.Cluster != clusterID {
			continue
		}
		if now.Sub(m.LastSeen) > s.ttl {
			continue
		}
		return true
	}
	return false
}

func (s *Store) NodesForCluster(clusterID string) []NodeInfo {
	if clusterID == "" {
		return nil
	}
	now := time.Now()

	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]NodeInfo, 0, 8)
	for _, m := range s.mastersByID {
		if m.Cluster != clusterID {
			continue
		}
		if now.Sub(m.LastSeen) > s.ttl {
			continue
		}
		out = append(out, m.Nodes...)
	}
	return out
}
