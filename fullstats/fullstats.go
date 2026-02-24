package fullstats

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.etcd.io/bbolt"
)

const (
	requestsBucket   = "requests"
	requestersBucket = "requesters"
	dbFileName       = "stats.db"
	asyncQueueSize   = 10000
)

// recordReq is a single request to record (for async processing).
type recordReq struct {
	key         string
	requesterIP string
	recordType  string
}

// RequestStats tracks statistics for a specific DNS request (domain + type).
type RequestStats struct {
	FirstSeen time.Time `json:"first_seen"`
	LastSeen  time.Time `json:"last_seen"`
	Count     uint64    `json:"count"`
}

// RequesterStats tracks statistics for a requester (IP address).
type RequesterStats struct {
	FirstSeen time.Time         `json:"first_seen"`
	TypeCount map[string]uint64 `json:"type_count"` // Map of record type -> count
}

// Tracker manages full statistics tracking using bbolt.
type Tracker struct {
	db        *bbolt.DB
	enabled   bool
	asyncCh   chan recordReq
	asyncWg   sync.WaitGroup
	closeOnce sync.Once
}

// New creates a new statistics tracker. If enabled is false, returns nil.
func New(statsDir string, enabled bool) (*Tracker, error) {
	if !enabled {
		return nil, nil
	}

	if err := os.MkdirAll(statsDir, 0o755); err != nil {
		return nil, fmt.Errorf("fullstats: create directory: %w", err)
	}

	dbPath := filepath.Join(statsDir, dbFileName)
	db, err := bbolt.Open(dbPath, 0o600, &bbolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("fullstats: open database: %w", err)
	}

	t := &Tracker{
		db:      db,
		enabled: true,
		asyncCh: make(chan recordReq, asyncQueueSize),
	}

	// Initialize buckets
	if err := t.initBuckets(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("fullstats: initialize buckets: %w", err)
	}

	t.asyncWg.Add(1)
	go t.asyncWorker()

	return t, nil
}

func (t *Tracker) asyncWorker() {
	defer t.asyncWg.Done()
	for req := range t.asyncCh {
		_ = t.recordRequestSync(req.key, req.requesterIP, req.recordType)
	}
}

func (t *Tracker) initBuckets() error {
	return t.db.Update(func(tx *bbolt.Tx) error {
		if _, err := tx.CreateBucketIfNotExists([]byte(requestsBucket)); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists([]byte(requestersBucket)); err != nil {
			return err
		}
		return nil
	})
}

// RecordRequest records a DNS request asynchronously so the DNS reply is not delayed by disk I/O.
// key should be in format "domain:type" (e.g., "example.com:A")
// requesterIP is the IP address of the client making the request.
// If the async queue is full, the record is dropped (non-blocking).
func (t *Tracker) RecordRequest(key string, requesterIP string, recordType string) error {
	if t == nil || !t.enabled {
		return nil
	}
	select {
	case t.asyncCh <- recordReq{key: key, requesterIP: requesterIP, recordType: recordType}:
	default:
		// Queue full; drop to avoid blocking the DNS path
	}
	return nil
}

// recordRequestSync does the actual bbolt write (used by async worker).
func (t *Tracker) recordRequestSync(key string, requesterIP string, recordType string) error {
	if t == nil || !t.enabled {
		return nil
	}

	now := time.Now()
	return t.db.Update(func(tx *bbolt.Tx) error {
		// Update request stats
		reqBucket := tx.Bucket([]byte(requestsBucket))
		if reqBucket == nil {
			return fmt.Errorf("requests bucket not found")
		}

		var stats RequestStats
		if data := reqBucket.Get([]byte(key)); data != nil {
			if err := json.Unmarshal(data, &stats); err != nil {
				return fmt.Errorf("unmarshal request stats: %w", err)
			}
		} else {
			stats.FirstSeen = now
		}

		stats.LastSeen = now
		stats.Count++

		statsData, err := json.Marshal(stats)
		if err != nil {
			return fmt.Errorf("marshal request stats: %w", err)
		}

		if err := reqBucket.Put([]byte(key), statsData); err != nil {
			return fmt.Errorf("put request stats: %w", err)
		}

		// Update requester stats
		reqrBucket := tx.Bucket([]byte(requestersBucket))
		if reqrBucket == nil {
			return fmt.Errorf("requesters bucket not found")
		}

		var reqrStats RequesterStats
		if data := reqrBucket.Get([]byte(requesterIP)); data != nil {
			if err := json.Unmarshal(data, &reqrStats); err != nil {
				return fmt.Errorf("unmarshal requester stats: %w", err)
			}
		} else {
			reqrStats.FirstSeen = now
			reqrStats.TypeCount = make(map[string]uint64)
		}

		if reqrStats.TypeCount == nil {
			reqrStats.TypeCount = make(map[string]uint64)
		}
		reqrStats.TypeCount[recordType]++

		reqrData, err := json.Marshal(reqrStats)
		if err != nil {
			return fmt.Errorf("marshal requester stats: %w", err)
		}

		if err := reqrBucket.Put([]byte(requesterIP), reqrData); err != nil {
			return fmt.Errorf("put requester stats: %w", err)
		}

		return nil
	})
}

// GetRequestStats retrieves statistics for a specific request.
func (t *Tracker) GetRequestStats(key string) (*RequestStats, error) {
	if t == nil || !t.enabled {
		return nil, nil
	}

	var stats *RequestStats
	err := t.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(requestsBucket))
		if bucket == nil {
			return fmt.Errorf("requests bucket not found")
		}

		data := bucket.Get([]byte(key))
		if data == nil {
			return nil
		}

		stats = &RequestStats{}
		return json.Unmarshal(data, stats)
	})

	return stats, err
}

// GetRequesterStats retrieves statistics for a specific requester.
func (t *Tracker) GetRequesterStats(requesterIP string) (*RequesterStats, error) {
	if t == nil || !t.enabled {
		return nil, nil
	}

	var stats *RequesterStats
	err := t.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(requestersBucket))
		if bucket == nil {
			return fmt.Errorf("requesters bucket not found")
		}

		data := bucket.Get([]byte(requesterIP))
		if data == nil {
			return nil
		}

		stats = &RequesterStats{}
		return json.Unmarshal(data, stats)
	})

	return stats, err
}

// GetAllRequests returns all request statistics.
func (t *Tracker) GetAllRequests() (map[string]*RequestStats, error) {
	if t == nil || !t.enabled {
		return nil, nil
	}

	result := make(map[string]*RequestStats)
	err := t.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(requestsBucket))
		if bucket == nil {
			return fmt.Errorf("requests bucket not found")
		}

		return bucket.ForEach(func(k, v []byte) error {
			var stats RequestStats
			if err := json.Unmarshal(v, &stats); err != nil {
				return err
			}
			result[string(k)] = &stats
			return nil
		})
	})

	return result, err
}

// GetAllRequesters returns all requester statistics.
func (t *Tracker) GetAllRequesters() (map[string]*RequesterStats, error) {
	if t == nil || !t.enabled {
		return nil, nil
	}

	result := make(map[string]*RequesterStats)
	err := t.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(requestersBucket))
		if bucket == nil {
			return fmt.Errorf("requesters bucket not found")
		}

		return bucket.ForEach(func(k, v []byte) error {
			var stats RequesterStats
			if err := json.Unmarshal(v, &stats); err != nil {
				return err
			}
			result[string(k)] = &stats
			return nil
		})
	})

	return result, err
}

// Close closes the async channel, waits for the worker to drain, then closes the database.
func (t *Tracker) Close() error {
	if t == nil || !t.enabled {
		return nil
	}
	var closeErr error
	t.closeOnce.Do(func() {
		close(t.asyncCh)
		t.asyncWg.Wait()
		closeErr = t.db.Close()
	})
	return closeErr
}
