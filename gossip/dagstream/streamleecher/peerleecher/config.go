package peerleecher

import (
	"time"

	"github.com/Fantom-foundation/go-opera/gossip/essteam"
)

type EpochDownloaderConfig struct {
	RecheckInterval        time.Duration
	DefaultChunkSize       essteam.Metric
	ParallelChunksDownload int
}
