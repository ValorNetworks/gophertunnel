package packet

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

func TestSetScoreBedrock12645WireFormat(t *testing.T) {
	// Fixed 1.26.45 payloads prevent a matching reader/writer bug from passing
	// a round-trip test. Removal has one optional marker, not two.
	tests := []struct {
		name    string
		entries []protocol.ScoreboardEntry
		wire    string
	}{
		{
			name: "remove named objective",
			entries: []protocol.ScoreboardEntry{{
				IdentityType: protocol.ScoreboardIdentityRemove,
				EntryID:      1, ObjectiveName: "Valor",
			}},
			wire: "\x01\x00\x06remove\x02\x01\x05Valor",
		},
		{
			name: "remove without objective",
			entries: []protocol.ScoreboardEntry{{
				IdentityType: protocol.ScoreboardIdentityRemove, EntryID: 1,
			}},
			wire: "\x01\x00\x06remove\x02\x00",
		},
		{
			name: "refresh multiple lobby lines",
			entries: []protocol.ScoreboardEntry{
				{IdentityType: protocol.ScoreboardIdentityRemove, EntryID: 0, ObjectiveName: "Valor"},
				{IdentityType: protocol.ScoreboardIdentityRemove, EntryID: 1, ObjectiveName: "Valor"},
				{IdentityType: protocol.ScoreboardIdentityFakePlayer, EntryID: 0, ObjectiveName: "Valor", DisplayName: "rank"},
			},
			wire: "\x03" +
				"\x00\x06remove\x00\x01\x05Valor" +
				"\x00\x06remove\x02\x01\x05Valor" +
				"\x03\x10changefakeplayer\x00\x05Valor\x00\x00\x00\x00\x04rank",
		},
		{
			name: "unscoped removal followed by another entry",
			entries: []protocol.ScoreboardEntry{
				{IdentityType: protocol.ScoreboardIdentityRemove, EntryID: 1},
				{IdentityType: protocol.ScoreboardIdentityRemove, EntryID: 2, ObjectiveName: "Valor"},
			},
			wire: "\x02\x00\x06remove\x02\x00\x00\x06remove\x04\x01\x05Valor",
		},
		{
			name: "add line remains unchanged",
			entries: []protocol.ScoreboardEntry{{
				IdentityType: protocol.ScoreboardIdentityFakePlayer,
				EntryID:      1, ObjectiveName: "Valor", Score: 1, DisplayName: "rank",
			}},
			wire: "\x01\x03\x10changefakeplayer\x02\x05Valor\x01\x00\x00\x00\x04rank",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Run("encode", func(t *testing.T) {
				var buffer bytes.Buffer
				writer := protocol.NewWriter(&buffer, 0)
				(&SetScore{Entries: tt.entries}).Marshal(writer)
				if err := writer.Err(); err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(buffer.Bytes(), []byte(tt.wire)) {
					t.Fatalf("encoded SetScore = %x, want %x", buffer.Bytes(), tt.wire)
				}
			})
			t.Run("decode", func(t *testing.T) {
				defer func() {
					if recovered := recover(); recovered != nil {
						t.Fatalf("decoding valid 1.26.45 SetScore panicked: %v", recovered)
					}
				}()
				buffer := bytes.NewBufferString(tt.wire)
				var decoded SetScore
				decoded.Marshal(protocol.NewReader(buffer, 0, true))
				if !reflect.DeepEqual(decoded.Entries, tt.entries) {
					t.Fatalf("decoded entries = %#v, want %#v", decoded.Entries, tt.entries)
				}
				if buffer.Len() != 0 {
					t.Fatalf("decoder left %d trailing bytes", buffer.Len())
				}
			})
		})
	}
}
