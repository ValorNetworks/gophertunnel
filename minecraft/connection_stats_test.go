package minecraft

import (
	"net"
	"testing"
	"time"

	"github.com/sandertv/go-raknet"
)

type statsTransport struct {
	net.Conn
	value raknet.ConnectionStats
}

func (s statsTransport) ConnectionStats() raknet.ConnectionStats { return s.value }

func TestConnectionStatsOptionalTransport(t *testing.T) {
	var unsupported Conn
	if _, ok := unsupported.ConnectionStats(); ok {
		t.Fatal("unsupported transport reports statistics")
	}
	now := time.Now()
	value := raknet.ConnectionStats{Count: 2, UpdatedAt: now, CapturedAt: now, OriginalTransmissions: 10, Retransmissions: 3}
	value.RTT[0], value.RTT[1] = 12345*time.Microsecond, 16*time.Millisecond
	conn := &Conn{conn: statsTransport{value: value}}
	got, ok := conn.ConnectionStats()
	if !ok || got.Count != 2 || got.RTT != value.RTT || got.CapturedAt != now || got.UpdatedAt != now || got.OriginalTransmissions != 10 || got.Retransmissions != 3 {
		t.Fatalf("adapted=%+v supported=%v", got, ok)
	}
	got.RTT[0] = 0
	again, _ := conn.ConnectionStats()
	if again.RTT[0] != value.RTT[0] {
		t.Fatal("snapshot mutation leaked")
	}
}
