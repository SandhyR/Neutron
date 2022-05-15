package main

import (
	"errors"
	"github.com/pelletier/go-toml"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/auth"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"golang.org/x/oauth2"
	"log"
	"os"
	"sync"
)

var antikb = false

var killaura = false

func main() {
	config := readConfig()
	token, err := auth.RequestLiveToken()
	if err != nil {
		panic(err)
	}
	src := auth.RefreshTokenSource(token)

	p, err := minecraft.NewForeignStatusProvider(config.Connection.RemoteAddress)
	if err != nil {
		panic(err)
	}
	listener, err := minecraft.ListenConfig{
		StatusProvider: p,
	}.Listen("raknet", config.Connection.LocalAddress)
	log.Println("Listening on " + config.Connection.LocalAddress)
	if err != nil {
		panic(err)
	}
	defer listener.Close()
	for {
		c, err := listener.Accept()
		if err != nil {
			panic(err)
		}
		go handleConn(c.(*minecraft.Conn), listener, config, src)
	}
}

// handleConn handles a new incoming minecraft.Conn from the minecraft.Listener passed.
func handleConn(conn *minecraft.Conn, listener *minecraft.Listener, config config, src oauth2.TokenSource) {
	clientdata := conn.ClientData()
	clientdata.CurrentInputMode = config.ClientData.InputMode
	clientdata.DeviceModel = config.ClientData.DeviceModel
	log.Println(config.ClientData.DeviceModel)
	serverConn, err := minecraft.Dialer{
		TokenSource: src,
		ClientData:  clientdata,
	}.Dial("raknet", config.Connection.RemoteAddress)
	if err != nil {
		panic(err)
	}
	var g sync.WaitGroup
	g.Add(2)
	go func() {
		if err := conn.StartGame(serverConn.GameData()); err != nil {
			panic(err)
		}
		g.Done()
	}()
	go func() {
		if err := serverConn.DoSpawn(); err != nil {
			panic(err)
		}
		g.Done()
	}()
	g.Wait()

	go func() {
		// serverbound (client -> server)
		defer listener.Disconnect(conn, "connection lost")
		defer serverConn.Close()
		for {
			pk, err := conn.ReadPacket()
			if err != nil {
				return
			}

			switch p := pk.(type) {
			case *packet.Text:
				switch p.Message {
				case "antikb":
					if antikb {
						antikb = false
					} else {
						antikb = true
					}
					continue
				case "killaura":
					if killaura {
						killaura = false
					} else {
						killaura = true
					}
					continue
				default:
					break
				}
			}
			if err := serverConn.WritePacket(pk); err != nil {
				if disconnect, ok := errors.Unwrap(err).(minecraft.DisconnectError); ok {
					_ = listener.Disconnect(conn, disconnect.Error())
				}
				return
			}
		}
	}()
	go func() {
		// clientbound (server -> client)
		defer serverConn.Close()
		defer listener.Disconnect(conn, "connection lost")
		for {
			pk, err := serverConn.ReadPacket()
			if err != nil {
				if disconnect, ok := errors.Unwrap(err).(minecraft.DisconnectError); ok {
					_ = listener.Disconnect(conn, disconnect.Error())
				}
				return
			}
			switch p := pk.(type) {
			case *packet.SetActorMotion:
				if p.EntityRuntimeID == conn.GameData().EntityRuntimeID {
					if antikb {
						continue
					}
				}
			case *packet.MoveActorAbsolute:
				pos := p.Position
				if killaura {
					conn.WritePacket(&packet.InventoryTransaction{
						TransactionData: &protocol.UseItemOnEntityTransactionData{
							TargetEntityRuntimeID: p.EntityRuntimeID,
							ActionType:            protocol.UseItemOnEntityActionAttack,
							HotBarSlot:            0,
							HeldItem:              protocol.ItemInstance{},
							Position:              pos,
						},
					})
				}
			default:
				break
			}
			if err := conn.WritePacket(pk); err != nil {
				return
			}
		}
	}()
}

type config struct {
	Connection struct {
		LocalAddress  string
		RemoteAddress string
	}
	ClientData struct {
		DeviceModel string
		InputMode   int
	}
}

func readConfig() config {
	c := config{}
	if _, err := os.Stat("config.toml"); os.IsNotExist(err) {
		f, err := os.Create("config.toml")
		if err != nil {
			log.Fatalf("error creating config: %v", err)
		}
		data, err := toml.Marshal(c)
		if err != nil {
			log.Fatalf("error encoding default config: %v", err)
		}
		if _, err := f.Write(data); err != nil {
			log.Fatalf("error writing encoded default config: %v", err)
		}
		_ = f.Close()
	}
	data, err := os.ReadFile("config.toml")
	if err != nil {
		log.Fatalf("error reading config: %v", err)
	}
	if err := toml.Unmarshal(data, &c); err != nil {
		log.Fatalf("error decoding config: %v", err)
	}
	if c.Connection.LocalAddress == "" {
		c.Connection.LocalAddress = "0.0.0.0:19132"
	}
	data, _ = toml.Marshal(c)
	if err := os.WriteFile("config.toml", data, 0644); err != nil {
		log.Fatalf("error writing config file: %v", err)
	}
	return c
}
