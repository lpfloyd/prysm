// Package kv defines a bolt-db, key-value store implementation
// of the Database interface defined by a Prysm beacon node.
package kv

import (
	"os"
	"path"
	"sync"
	"time"

	"github.com/dgraph-io/ristretto"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	prombolt "github.com/prysmaticlabs/prombbolt"
	"github.com/prysmaticlabs/prysm/beacon-chain/cache"
	"github.com/prysmaticlabs/prysm/beacon-chain/db/iface"
	bolt "go.etcd.io/bbolt"
)

var _ = iface.Database(&Store{})

const (
	// VotesCacheSize with 1M validators will be 8MB.
	VotesCacheSize = 1 << 23
	// NumOfVotes specifies the vote cache size.
	NumOfVotes       = 1 << 20
	databaseFileName = "beaconchain.db"
	boltAllocSize    = 8 * 1024 * 1024
)

// BlockCacheSize specifies 1000 slots worth of blocks cached, which
// would be approximately 2MB
var BlockCacheSize = int64(1 << 21)

// Store defines an implementation of the Prysm Database interface
// using BoltDB as the underlying persistent kv-store for eth2.
type Store struct {
	db                  *bolt.DB
	databasePath        string
	blockCache          *ristretto.Cache
	validatorIndexCache *ristretto.Cache
	stateSlotBitLock    sync.Mutex
	blockSlotBitLock    sync.Mutex
	stateSummaryCache   *cache.StateSummaryCache
}

// NewKVStore initializes a new boltDB key-value store at the directory
// path specified, creates the kv-buckets based on the schema, and stores
// an open connection db object as a property of the Store struct.
func NewKVStore(dirPath string, stateSummaryCache *cache.StateSummaryCache) (*Store, error) {
	if err := os.MkdirAll(dirPath, 0700); err != nil {
		return nil, err
	}
	datafile := path.Join(dirPath, databaseFileName)
	boltDB, err := bolt.Open(datafile, 0600, &bolt.Options{Timeout: 1 * time.Second, InitialMmapSize: 10e6})
	if err != nil {
		if err == bolt.ErrTimeout {
			return nil, errors.New("cannot obtain database lock, database may be in use by another process")
		}
		return nil, err
	}
	boltDB.AllocSize = boltAllocSize
	blockCache, err := ristretto.NewCache(&ristretto.Config{
		NumCounters: 1000,           // number of keys to track frequency of (1000).
		MaxCost:     BlockCacheSize, // maximum cost of cache (1000 Blocks).
		BufferItems: 64,             // number of keys per Get buffer.
	})
	if err != nil {
		return nil, err
	}

	validatorCache, err := ristretto.NewCache(&ristretto.Config{
		NumCounters: NumOfVotes,     // number of keys to track frequency of (1M).
		MaxCost:     VotesCacheSize, // maximum cost of cache (8MB).
		BufferItems: 64,             // number of keys per Get buffer.
	})
	if err != nil {
		return nil, err
	}

	kv := &Store{
		db:                  boltDB,
		databasePath:        dirPath,
		blockCache:          blockCache,
		validatorIndexCache: validatorCache,
		stateSummaryCache:   stateSummaryCache,
	}

	if err := kv.db.Update(func(tx *bolt.Tx) error {
		return createBuckets(
			tx,
			attestationsBucket,
			blocksBucket,
			stateBucket,
			proposerSlashingsBucket,
			attesterSlashingsBucket,
			voluntaryExitsBucket,
			chainMetadataBucket,
			checkpointBucket,
			archivedValidatorSetChangesBucket,
			archivedCommitteeInfoBucket,
			archivedBalancesBucket,
			archivedValidatorParticipationBucket,
			powchainBucket,
			stateSummaryBucket,
			archivedIndexRootBucket,
			slotsHasObjectBucket,
			// Indices buckets.
			attestationHeadBlockRootBucket,
			attestationSourceRootIndicesBucket,
			attestationSourceEpochIndicesBucket,
			attestationTargetRootIndicesBucket,
			attestationTargetEpochIndicesBucket,
			blockSlotIndicesBucket,
			blockParentRootIndicesBucket,
			finalizedBlockRootsIndexBucket,
			// New State Management service bucket.
			newStateServiceCompatibleBucket,
		)
	}); err != nil {
		return nil, err
	}

	err = prometheus.Register(createBoltCollector(kv.db))

	return kv, err
}

// ClearDB removes the previously stored database in the data directory.
func (k *Store) ClearDB() error {
	if _, err := os.Stat(k.databasePath); os.IsNotExist(err) {
		return nil
	}
	prometheus.Unregister(createBoltCollector(k.db))
	return os.Remove(path.Join(k.databasePath, databaseFileName))
}

// Close closes the underlying BoltDB database.
func (k *Store) Close() error {
	prometheus.Unregister(createBoltCollector(k.db))
	return k.db.Close()
}

// DatabasePath at which this database writes files.
func (k *Store) DatabasePath() string {
	return k.databasePath
}

func createBuckets(tx *bolt.Tx, buckets ...[]byte) error {
	for _, bucket := range buckets {
		if _, err := tx.CreateBucketIfNotExists(bucket); err != nil {
			return err
		}
	}
	return nil
}

// createBoltCollector returns a prometheus collector specifically configured for boltdb.
func createBoltCollector(db *bolt.DB) prometheus.Collector {
	return prombolt.New("boltDB", db)
}
