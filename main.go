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
	"strings"
	"sync"
)

var antikb = false
var killaura = false
var haste = false
var slowfalling = false
var noclip = false
var nightvision = false
var MessagePrefix = "§o§l§6Neutron§r§7 > "
var PREFIX = "/."

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
			case *packet.CommandRequest:
				var message = p.CommandLine
				var msg = strings.ToLower(message)
				var args = strings.Split(strings.TrimPrefix(msg, PREFIX), " ")
				var cmd = args[0]
				switch cmd {
				case "help":
					sendMessage(conn, `§aHelp Commands
§8§l• §r§7/.antikb
§8§l• §r§7/.killaura
§8§l• §r§7/.gamemode <type>
§8§l• §r§7/.haste
§8§l• §r§7/.slowfalling
§8§l• §r§7/.nightvision
§8§l• §r§7/.noclip
`)
					continue
				case "antikb":
					if antikb {
						antikb = false
						sendMessage(conn, "§aAnti Knockback has been turned off!")
					} else {
						antikb = true
						sendMessage(conn, "§aAnti Knockback has been turned on!")
					}
					continue
				case "killaura":
					if killaura {
						killaura = false
						sendMessage(conn, "§aKill Aura has been turned off!")
					} else {
						killaura = true
						sendMessage(conn, "§aKill Aura has been turned on!")
					}
					continue
				case "gamemode":
					switch args[1] {
					case "0":
						_ = conn.WritePacket(&packet.SetPlayerGameType{GameType: packet.GameTypeSurvival})
						sendMessage(conn, "§aSet own game mode to Survival!")
						continue
					case "s":
						_ = conn.WritePacket(&packet.SetPlayerGameType{GameType: packet.GameTypeSurvival})
						sendMessage(conn, "§aSet own game mode to Survival!")
						continue
					case "survival":
						_ = conn.WritePacket(&packet.SetPlayerGameType{GameType: packet.GameTypeSurvival})
						sendMessage(conn, "§aSet own game mode to Survival!")
						continue
					case "1":
						_ = conn.WritePacket(&packet.SetPlayerGameType{GameType: packet.GameTypeCreative})
						sendMessage(conn, "§aSet own game mode to Creative!")
						continue
					case "c":
						_ = conn.WritePacket(&packet.SetPlayerGameType{GameType: packet.GameTypeCreative})
						sendMessage(conn, "§aSet own game mode to Creative!")
						continue
					case "creative":
						_ = conn.WritePacket(&packet.SetPlayerGameType{GameType: packet.GameTypeCreative})
						sendMessage(conn, "§aSet own game mode to Creative!")
						continue
					case "2":
						_ = conn.WritePacket(&packet.SetPlayerGameType{GameType: packet.GameTypeAdventure})
						sendMessage(conn, "§aSet own game mode to Adventure!")
						continue
					case "a":
						_ = conn.WritePacket(&packet.SetPlayerGameType{GameType: packet.GameTypeAdventure})
						sendMessage(conn, "§aSet own game mode to Adventure!")
						continue
					case "adventure":
						_ = conn.WritePacket(&packet.SetPlayerGameType{GameType: packet.GameTypeAdventure})
						sendMessage(conn, "§aSet own game mode to Adventure!")
						continue
					default:
						sendMessage(conn, "§cUsage: "+PREFIX+"gamemode <mode>")
						break
					}
					continue
				case "haste":
					if haste {
						haste = false
						_ = conn.WritePacket(&packet.MobEffect{
							EntityRuntimeID: conn.GameData().EntityRuntimeID,
							Operation:       packet.MobEffectAdd,
							EffectType:      packet.EffectHaste,
							Amplifier:       2,
							Particles:       false,
							Duration:        1,
						})
						sendMessage(conn, "§aHaste has been turned off!")
					} else {
						haste = true
						_ = conn.WritePacket(&packet.MobEffect{
							EntityRuntimeID: conn.GameData().EntityRuntimeID,
							Operation:       packet.MobEffectAdd,
							EffectType:      packet.EffectHaste,
							Amplifier:       2,
							Particles:       false,
							Duration:        999999999,
						})
						sendMessage(conn, "§aHaste has been turned on!")
					}
					continue
				case "slowfalling":
					if slowfalling {
						slowfalling = false
						_ = conn.WritePacket(&packet.MobEffect{
							EntityRuntimeID: conn.GameData().EntityRuntimeID,
							Operation:       packet.MobEffectAdd,
							EffectType:      27,
							Amplifier:       2,
							Particles:       false,
							Duration:        1,
						})
						sendMessage(conn, "§aSlow Falling has been turned off!")
					} else {
						slowfalling = true
						_ = conn.WritePacket(&packet.MobEffect{
							EntityRuntimeID: conn.GameData().EntityRuntimeID,
							Operation:       packet.MobEffectAdd,
							EffectType:      27,
							Amplifier:       2,
							Particles:       false,
							Duration:        999999999,
						})
						sendMessage(conn, "§aSlow Falling has been turned on!")
					}
					continue
				case "noclip":
					if noclip {
						noclip = false
						_ = conn.WritePacket(&packet.AdventureSettings{
							Flags:                   packet.AdventureFlagNoClip,
							CommandPermissionLevel:  0,
							ActionPermissions:       0,
							PermissionLevel:         1,
							CustomStoredPermissions: 0,
							PlayerUniqueID:          conn.GameData().EntityUniqueID,
						})
						sendMessage(conn, "§aNo Clip has been turned off!")
					} else {
						noclip = true
						_ = conn.WritePacket(&packet.AdventureSettings{
							Flags:                   packet.AdventureFlagNoClip,
							CommandPermissionLevel:  0,
							ActionPermissions:       0x128,
							PermissionLevel:         1,
							CustomStoredPermissions: 0,
							PlayerUniqueID:          conn.GameData().EntityUniqueID,
						})
						sendMessage(conn, "§aNo Clip has been turned on!")
					}
					continue
				case "nightvision":
					if nightvision {
						nightvision = false
						_ = conn.WritePacket(&packet.MobEffect{
							EntityRuntimeID: conn.GameData().EntityRuntimeID,
							Operation:       packet.MobEffectAdd,
							EffectType:      packet.EffectNightVision,
							Amplifier:       2,
							Particles:       false,
							Duration:        1,
						})
						sendMessage(conn, "§aNight Vision has been turned off!")
					} else {
						nightvision = true
						_ = conn.WritePacket(&packet.MobEffect{
							EntityRuntimeID: conn.GameData().EntityRuntimeID,
							Operation:       packet.MobEffectAdd,
							EffectType:      packet.EffectNightVision,
							Amplifier:       2,
							Particles:       false,
							Duration:        999999999,
						})
						sendMessage(conn, "§aNight Vision has been turned on!")
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
					_ = conn.WritePacket(&packet.InventoryTransaction{
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

func sendMessage(conn *minecraft.Conn, message string) {
	_ = conn.WritePacket(&packet.Text{
		TextType:         packet.TextTypeRaw,
		NeedsTranslation: false,
		SourceName:       "",
		Message:          MessagePrefix + message,
		Parameters:       nil,
		XUID:             "",
		PlatformChatID:   "",
	})
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
