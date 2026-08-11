package storage

import (
	"encoding/json"
	"fmt"
	"time"

	"go.etcd.io/bbolt"

	"github.com/gatra-io/gatra/internal/errors"
)

var trajectoryBucket = []byte("trajectories")

type RuleStateData struct {
	Sum         float64             `json:"sum"`
	Count       int64               `json:"count"`
	UniqueSet   map[string]struct{} `json:"unique_set"`
	WindowStart time.Time           `json:"window_start"`
}

type TrajectoryData struct {
	LastAccess time.Time                `json:"last_access"`
	Rules      map[string]RuleStateData `json:"rules"`
}

type Store struct {
	db *bbolt.DB
}

// NewStore opens or creates an embedded bbolt database file on disk.
func NewStore(dbPath string) (*Store, error) {
	db, err := bbolt.Open(dbPath, 0600, &bbolt.Options{Timeout: 2 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("%w: failed to open bbolt db '%s': %v", errors.ErrInvalidConfiguration, dbPath, err)
	}

	err = db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(trajectoryBucket)
		return err
	})
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to create storage bucket: %w", err)
	}

	return &Store{db: db}, nil
}

// Close gracefully flushes and unlocks the database file.
func (s *Store) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// SaveTrajectory persists a trajectory session state to disk.
func (s *Store) SaveTrajectory(trajectoryID string, data TrajectoryData) error {
	bytes, err := json.Marshal(data)
	if err != nil {
		return err
	}

	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(trajectoryBucket)
		return b.Put([]byte(trajectoryID), bytes)
	})
}

// LoadAllTrajectories restores active trajectory states from disk into RAM on startup.
func (s *Store) LoadAllTrajectories() (map[string]TrajectoryData, error) {
	result := make(map[string]TrajectoryData)

	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(trajectoryBucket)
		return b.ForEach(func(k, v []byte) error {
			var data TrajectoryData
			if err := json.Unmarshal(v, &data); err != nil {
				return err
			}
			result[string(k)] = data
			return nil
		})
	})

	return result, err
}

// DeleteTrajectory removes a expired session record from disk.
func (s *Store) DeleteTrajectory(trajectoryID string) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(trajectoryBucket)
		return b.Delete([]byte(trajectoryID))
	})
}