package main

import (
	"encoding/json"
	"errors"
	"flag"
	"log"
	"os"

	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/auth"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"golang.org/x/oauth2"
)

const tokenFile = "auth.tok"

var (
	localAddress  = flag.String("local", "0.0.0.0:19132", "address the proxy listens on")
	remoteAddress = flag.String("remote", "", "address of the server to forward players to")
)

func main() {
	flag.Parse()
	if *remoteAddress == "" {
		flag.Usage()
		log.Fatal("the -remote flag is required")
	}
	src := tokenSource()

	p, err := minecraft.NewForeignStatusProvider(*remoteAddress)
	if err != nil {
		panic(err)
	}
	listener, err := minecraft.ListenConfig{
		StatusProvider:     p,
		LowLevelMode:       true,
		EnableBatchReading: true,
		FlushRate:          -1,
	}.Listen("raknet", *localAddress)
	if err != nil {
		panic(err)
	}
	defer listener.Close()
	for {
		c, err := listener.Accept()
		if err != nil {
			panic(err)
		}
		go handleConn(c.(*minecraft.Conn), listener, src)
	}
}

func tokenSource() oauth2.TokenSource {
	token := new(oauth2.Token)
	data, err := os.ReadFile(tokenFile)
	if err == nil && json.Unmarshal(data, token) == nil {
		src := auth.RefreshTokenSource(token)
		if current, err := src.Token(); err == nil {
			saveToken(current)
			return src
		}
		log.Println("cached token no longer works, requesting a new login")
	}
	token, err = auth.RequestLiveToken()
	if err != nil {
		log.Fatalf("request live token: %v", err)
	}
	saveToken(token)
	return auth.RefreshTokenSource(token)
}

func saveToken(t *oauth2.Token) {
	data, err := json.Marshal(t)
	if err != nil {
		log.Printf("cache token: encode: %v", err)
		return
	}
	if err := os.WriteFile(tokenFile, data, 0600); err != nil {
		log.Printf("cache token: write: %v", err)
	}
}

func handleConn(conn *minecraft.Conn, listener *minecraft.Listener, src oauth2.TokenSource) {
	serverConn, err := minecraft.Dialer{
		TokenSource:        src,
		ClientData:         conn.ClientData(),
		LowLevelMode:       true,
		EnableBatchReading: true,
		FlushRate:          -1,
	}.Dial("raknet", *remoteAddress)
	if err != nil {
		panic(err)
	}

	go func() {
		defer listener.Disconnect(conn, "connection lost")
		defer serverConn.Close()
		for {
			pks, err := conn.ReadBatch()
			if err != nil {
				return
			}
			for _, pk := range pks {
				if err := serverConn.WritePacket(pk); err != nil {
					var disc minecraft.DisconnectError
					if ok := errors.As(err, &disc); ok {
						_ = listener.Disconnect(conn, disc.Error())
					}
					return
				}
			}
			if err := serverConn.Flush(); err != nil {
				return
			}
		}
	}()
	go func() {
		defer serverConn.Close()
		defer listener.Disconnect(conn, "connection lost")
		for {
			pks, err := serverConn.ReadBatch()
			if err != nil {
				var disc minecraft.DisconnectError
				if ok := errors.As(err, &disc); ok {
					_ = listener.Disconnect(conn, disc.Error())
				}
				return
			}
			for _, pk := range pks {
				if registry, ok := pk.(*packet.ItemRegistry); ok {
					for _, item := range registry.Items {
						if item.Name == "minecraft:shield" {
							conn.SetShieldID(int32(item.RuntimeID))
							serverConn.SetShieldID(int32(item.RuntimeID))
						}
					}
				}
				if err := conn.WritePacket(pk); err != nil {
					return
				}
			}
			if err := conn.Flush(); err != nil {
				return
			}
		}
	}()
}
