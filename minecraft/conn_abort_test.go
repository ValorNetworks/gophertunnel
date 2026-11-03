package minecraft

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type abortingProtocol struct {
	proto
	cause error
}

func (p abortingProtocol) ConvertFromLatest(pk packet.Packet, conn *Conn) []packet.Packet {
	conn.Abort(p.cause)
	// Even a converter returning a replacement after cancellation must not
	// queue or encode it, nor report a successful write.
	return []packet.Packet{pk}
}

func TestConnAbortFromProtocolConversion(t *testing.T) {
	for name, write := range map[string]func(*Conn, packet.Packet) error{
		"buffered":    (*Conn).WritePacket,
		"ack":         func(c *Conn, pk packet.Packet) error { return c.FlushSingleWithACK(pk, 1) },
		"reliability": func(c *Conn, pk packet.Packet) error { return c.FlushSingleWithReliability(pk, 2) },
	} {
		t.Run(name, func(t *testing.T) {
			local, remote := net.Pipe()
			t.Cleanup(func() { _ = local.Close(); _ = remote.Close() })
			cause := errors.New("conversion failed")
			conn := newConn(local, nil, slog.Default(), abortingProtocol{cause: cause}, 0, false)
			done := make(chan error, 1)
			go func() { done <- write(conn, &packet.Unknown{PacketID: 200, Payload: []byte{1}}) }()
			select {
			case err := <-done:
				if !errors.Is(err, cause) {
					t.Fatalf("write error = %v, want original conversion error", err)
				}
			case <-time.After(time.Second):
				t.Fatal("conversion abort waited for the packet-writing lock")
			}
			conn.sendMu.Lock()
			queued := len(conn.bufferedSend)
			conn.sendMu.Unlock()
			if queued != 0 {
				t.Fatal("aborted conversion queued a packet")
			}
			if err := conn.WritePacket(&packet.Unknown{PacketID: 200}); !errors.Is(err, cause) {
				t.Fatalf("later write = %v, want original conversion error", err)
			}
			if err := conn.Close(); err != nil && !errors.Is(err, cause) {
				t.Fatal(err)
			}
		})
	}
}

type delayedCloseConn struct {
	net.Conn
	started chan struct{}
	release chan struct{}
	calls   atomic.Int32
}

func (c *delayedCloseConn) Close() error {
	if c.calls.Add(1) == 1 {
		close(c.started)
	}
	<-c.release
	return c.Conn.Close()
}

func TestConnAbortDoesNotWaitForTransportOrGracefulClose(t *testing.T) {
	local, remote := net.Pipe()
	transport := &delayedCloseConn{Conn: local, started: make(chan struct{}), release: make(chan struct{})}
	t.Cleanup(func() { _ = local.Close(); _ = remote.Close() })
	conn := newConn(transport, nil, slog.Default(), proto{}, 0, false)
	// Model a graceful close already owning once while waiting for a writer.
	conn.sendMu.Lock()
	closing := make(chan error, 1)
	go func() { closing <- conn.Close() }()
	deadline := time.Now().Add(time.Second)
	for conn.encMu.TryLock() {
		conn.encMu.Unlock()
		if time.Now().After(deadline) {
			conn.sendMu.Unlock()
			t.Fatal("graceful close did not start flushing")
		}
		time.Sleep(time.Millisecond)
	}
	cause := errors.New("invalid registry")
	conn.Abort(cause)
	if !errors.Is(context.Cause(conn.ctx), cause) {
		t.Fatal("abort did not synchronously preserve its cause")
	}
	var callers sync.WaitGroup
	for range 32 {
		callers.Go(func() { conn.Abort(errors.New("later failure")) })
	}
	callers.Wait()
	select {
	case <-transport.started:
	case <-time.After(time.Second):
		t.Fatal("abort waited for graceful close")
	}
	// The network close is still blocked, but every abort request has returned.
	if got := transport.calls.Load(); got != 1 {
		t.Fatalf("concurrent aborts scheduled %d transport closes", got)
	}
	close(transport.release)
	conn.sendMu.Unlock()
	select {
	case <-closing:
	case <-time.After(time.Second):
		t.Fatal("graceful close did not finish after abort")
	}
}

func TestConnAbortNilCause(t *testing.T) {
	local, remote := net.Pipe()
	t.Cleanup(func() { _ = local.Close(); _ = remote.Close() })
	conn := newConn(local, nil, slog.Default(), proto{}, 0, false)
	conn.Abort(nil)
	if !errors.Is(context.Cause(conn.ctx), net.ErrClosed) {
		t.Fatal(context.Cause(conn.ctx))
	}
	_ = conn.Close()
}
