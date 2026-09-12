package minecraft

import (
	"time"

	"github.com/sandertv/go-raknet"
)

// ConnectionStats is a copied transport snapshot. RTT contains up to 64 clean
// full send-to-ACK timings in ACK order, observed within ten seconds of CapturedAt.
// The transmission counters cover ten one-second buckets, including the current
// partial bucket. Retransmissions are sent datagrams, not a packet-loss estimate.
type ConnectionStats struct {
	RTT                   [64]time.Duration
	Count                 int
	UpdatedAt             time.Time
	CapturedAt            time.Time
	OriginalTransmissions uint64
	Retransmissions       uint64
}

// ConnectionStatsSource is an optional read-only capability for transports and
// connection wrappers. It must not wait on gameplay or socket-write locks.
type ConnectionStatsSource interface {
	ConnectionStats() (ConnectionStats, bool)
}

// ConnectionStats reports passive transport observations without sending probes.
// Non-RakNet transports without this capability return unavailable.
func (conn *Conn) ConnectionStats() (ConnectionStats, bool) {
	if source, ok := conn.conn.(interface{ ConnectionStats() raknet.ConnectionStats }); ok {
		value := source.ConnectionStats()
		return ConnectionStats{RTT: value.RTT, Count: value.Count, UpdatedAt: value.UpdatedAt,
			CapturedAt: value.CapturedAt, OriginalTransmissions: value.OriginalTransmissions,
			Retransmissions: value.Retransmissions}, true
	}
	return ConnectionStats{}, false
}
