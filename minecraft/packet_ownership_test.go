package minecraft

import (
	"bytes"
	"context"
	"io"
	"net"
	"testing"

	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

func TestReceiveRetainsOwnedPacketData(t *testing.T) {
	for _, loggedIn := range []bool{false, true} {
		t.Run(map[bool]string{false: "deferred", true: "queued"}[loggedIn], func(t *testing.T) {
			conn := &Conn{ctx: context.Background(), loggedIn: loggedIn, packets: make(chan *packetData, 8)}
			conn.expectedIDs.Store([]uint32{})
			data := ownershipTestPacket("retained")
			want := bytes.Clone(data)
			if err := conn.receive(data); err != nil {
				t.Fatal(err)
			}
			clear(data)
			got, err := conn.ReadBytes()
			if err != nil || !bytes.Equal(got, want) {
				t.Fatalf("retained packet = %x, %v; want %x", got, err, want)
			}
		})
	}
}

func TestBatchReadingPreservesFramesAcrossDecoderReuse(t *testing.T) {
	first, second := ownershipTestPacket("first"), ownershipTestPacket("second")
	batch := []byte{0xfe, byte(len(first))}
	batch = append(batch, first...)
	batch = append(batch, byte(len(second)))
	batch = append(batch, second...)
	reader := &reusedBatchReader{data: bytes.Clone(batch)}
	decoder := packet.NewDecoder(reader)
	conn := &Conn{ctx: context.Background(), loggedIn: true, batchMode: true, batchPackets: make(chan []*packetData, 1)}
	conn.expectedIDs.Store([]uint32{})
	frames, err := decoder.Decode()
	if err != nil {
		t.Fatal(err)
	}
	if err := conn.receiveMultiple(frames); err != nil {
		t.Fatal(err)
	}
	clear(reader.data)
	got, err := conn.ReadBatchBytes()
	if err != nil || len(got) != 2 || !bytes.Equal(got[0], first) || !bytes.Equal(got[1], second) {
		t.Fatalf("batch lost ownership or grouping: %x, %v", got, err)
	}
}

func TestPacketObserverRetainsOwnedPayload(t *testing.T) {
	conn := newBufferedTestConn(t)
	var observed []byte
	conn.packetFunc = func(_ packet.Header, data []byte, _, _ net.Addr) { observed = data }
	data := ownershipTestPacket("observer")
	if _, err := parseData(data, conn); err != nil {
		t.Fatal(err)
	}
	clear(data)
	if string(observed) != "observer" {
		t.Fatalf("observer retained reused buffer: %q", observed)
	}
}

func ownershipTestPacket(payload string) []byte {
	var buf bytes.Buffer
	_ = (&packet.Header{PacketID: 200}).Write(&buf)
	buf.WriteString(payload)
	return buf.Bytes()
}

type reusedBatchReader struct{ data []byte }

func (r *reusedBatchReader) ReadPacket() ([]byte, error) { return r.data, nil }
func (r *reusedBatchReader) Read([]byte) (int, error)    { return 0, io.EOF }
