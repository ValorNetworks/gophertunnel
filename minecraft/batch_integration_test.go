package minecraft

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/sandertv/go-raknet"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

func TestLoginAndPacketExchangeWithBatchReading(t *testing.T) {
	for _, serverBatch := range []bool{false, true} {
		for _, clientBatch := range []bool{false, true} {
			t.Run(fmt.Sprintf("server=%t/client=%t", serverBatch, clientBatch), func(t *testing.T) {
				ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
				defer cancel()
				log := slog.New(slog.NewTextHandler(io.Discard, nil))
				listener, err := (ListenConfig{AuthenticationDisabled: true, EnableBatchReading: serverBatch, LoginTimeout: 5 * time.Second, ErrorLog: log}).Listen("raknet", "127.0.0.1:0")
				if err != nil {
					t.Fatal(err)
				}
				defer listener.Close()
				accepted := make(chan *Conn, 1)
				started := make(chan error, 1)
				go func() {
					netConn, err := listener.Accept()
					if err != nil {
						started <- err
						return
					}
					conn := netConn.(*Conn)
					accepted <- conn
					started <- conn.StartGameContext(ctx, GameData{})
				}()
				client, err := (Dialer{EnableBatchReading: clientBatch, ErrorLog: log}).DialContext(ctx, "raknet", listener.Addr().String())
				if err != nil {
					t.Fatal(err)
				}
				defer client.Close()
				var server *Conn
				select {
				case server = <-accepted:
				case <-ctx.Done():
					t.Fatal(ctx.Err())
				}
				defer server.Close()
				if err := client.DoSpawn(); err != nil {
					t.Fatal(err)
				}
				select {
				case err := <-started:
					if err != nil {
						t.Fatal(err)
					}
				case <-ctx.Done():
					t.Fatal(ctx.Err())
				}
				for _, conn := range []*Conn{client, server} {
					if err := conn.SetReadDeadline(time.Now().Add(3 * time.Second)); err != nil {
						t.Fatal(err)
					}
				}
				for _, message := range []string{"first", "second"} {
					if err := client.WritePacket(&packet.Text{TextType: packet.TextTypeChat, Message: message}); err != nil {
						t.Fatal(err)
					}
				}
				if err := client.Flush(); err != nil {
					t.Fatal(err)
				}
				readBatchTestText(t, server, serverBatch, []string{"first", "second"})
				if err := server.WritePacket(&packet.Text{TextType: packet.TextTypeChat, Message: "reply"}); err != nil {
					t.Fatal(err)
				}
				if err := server.Flush(); err != nil {
					t.Fatal(err)
				}
				readBatchTestText(t, client, clientBatch, []string{"reply"})
			})
		}
	}
}

func readBatchTestText(t *testing.T, conn *Conn, batch bool, want []string) {
	t.Helper()
	var got []string
	for len(got) < len(want) {
		var packets []packet.Packet
		if batch {
			var err error
			packets, err = conn.ReadBatch()
			if err != nil {
				t.Fatal(err)
			}
		} else {
			pk, err := conn.ReadPacket()
			if err != nil {
				t.Fatal(err)
			}
			packets = []packet.Packet{pk}
		}
		for _, pk := range packets {
			if text, ok := pk.(*packet.Text); ok {
				got = append(got, text.Message)
			}
		}
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("text messages = %q, want %q", got, want)
	}
}

func TestLoginTimeoutClosesIdleTransportInBothReadingModes(t *testing.T) {
	for _, batch := range []bool{false, true} {
		t.Run(fmt.Sprint(batch), func(t *testing.T) {
			log := slog.New(slog.NewTextHandler(io.Discard, nil))
			listener, err := (ListenConfig{AuthenticationDisabled: true, EnableBatchReading: batch, LoginTimeout: 50 * time.Millisecond, ErrorLog: log}).Listen("raknet", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer listener.Close()
			conn, err := raknet.DialTimeout(listener.Addr().String(), 2*time.Second)
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			read := make(chan error, 1)
			go func() {
				_, err := conn.Read(make([]byte, 1024))
				read <- err
			}()
			select {
			case err := <-read:
				if err == nil || conn.Context().Err() == nil {
					t.Fatalf("login deadline did not close transport: %v", err)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("idle transport stayed open after login deadline")
			}
		})
	}
}
