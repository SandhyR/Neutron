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
	"strconv"
	"strings"
	"sync"
	"time"
)

var players = map[string]*Player{}
var reach float32 = 0.0
var fly = false
var antikb = false
var jumpboost = false
var speed = false
var killaura = false
var haste = false
var slowfalling = false
var noclip = false
var nightvision = false
var MessagePrefix = "§o§l§6Neutron§r§7 > "
var PREFIX = "/."

type Player struct {
	name          string
	runtimeid     uint64
	uniqueid      int64
	dirtymetadata map[uint32]any
	metadata      map[uint32]any
}

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
			case *packet.PlayerAuthInput:
				p.InputMode = config.AuthInput.InputMode
				break
			case *packet.RequestAbility:
				//TODO: use this so that flying is not detected
				//https://github.com/pmmp/PocketMine-MP/blob/4ec97d0f7ae84270abc77f02fc57b4f60d1ba87d/src/network/mcpe/handler/InGamePacketHandler.php#L974
				if p.Ability == packet.AbilityFlying {
					if p.Value == fly {
						continue
					}
				}
				break
			case *packet.CommandRequest:
				var message = p.CommandLine
				var msg = strings.ToLower(message)
				var args = strings.Split(strings.TrimPrefix(msg, PREFIX), " ")
				var cmd = args[0]
				switch cmd {
				case "help":
					sendMessage(conn, `§aHelp Commands §8§l• §r§7/.antikb
					§8§l• §r§7/.killaura
					§8§l• §r§7/.gamemode <type>
					§8§l• §r§7/.haste
					§8§l• §r§7/.slowfalling
					§8§l• §r§7/.nightvision
					§8§l• §r§7/.speed
					§8§l• §r§7/.jumpboost
					§8§l• §r§7/.noclip`)
					continue
				case "fly":
					if fly {
						fly = false
						_ = conn.WritePacket(&packet.AdventureSettings{
							Flags:                   0 & packet.AdventureFlagAllowFlight,
							CommandPermissionLevel:  0,
							ActionPermissions:       0,
							PermissionLevel:         1,
							CustomStoredPermissions: 0,
							PlayerUniqueID:          conn.GameData().EntityUniqueID,
						})
						sendMessage(conn, "§aFly has been turned off!")
					} else {
						fly = true
						_ = conn.WritePacket(&packet.AdventureSettings{
							Flags:                   0 | packet.AdventureFlagAllowFlight,
							CommandPermissionLevel:  0,
							ActionPermissions:       0,
							PermissionLevel:         1,
							CustomStoredPermissions: 0,
							PlayerUniqueID:          conn.GameData().EntityUniqueID,
						})
						sendMessage(conn, "§aFly has been turned on!")
					}
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
					if len(args) < 3 && len(args) > 1 {
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
							sendMessage(conn, "§cUnknown \""+args[1]+"\" game mode!")
							break
						}
					} else {
						sendMessage(conn, "§cUsage: "+PREFIX+"gamemode <mode>")
					}
					continue
				case "reach":
					if len(args) > 1 {
						nreach, err := strconv.ParseFloat(args[1], 32)
						if err != nil {
							panic(err)
						}
						reach = float32(nreach)
						if args[1] == "0" {
							for _, player := range players {
								player.dirtymetadata = player.metadata
								syncActor(conn, player.runtimeid, player.metadata)
							}
							sendMessage(conn, "Successfully reset reach")
							continue
						}

						setReach(conn, float32(nreach))
						sendMessage(conn, "Successfully set reach "+args[1])
					}
					continue
				case "haste":
					if haste {
						haste = false
						_ = conn.WritePacket(&packet.MobEffect{
							EntityRuntimeID: conn.GameData().EntityRuntimeID,
							Operation:       packet.MobEffectRemove,
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
				case "speed":
					if speed {
						speed = false
						_ = conn.WritePacket(&packet.MobEffect{
							EntityRuntimeID: conn.GameData().EntityRuntimeID,
							Operation:       packet.MobEffectRemove,
							EffectType:      packet.EffectSpeed,
							Amplifier:       2,
							Particles:       false,
							Duration:        1,
						})
						sendMessage(conn, "§aSpeed has been turned off!")
					} else {
						speed = true
						_ = conn.WritePacket(&packet.MobEffect{
							EntityRuntimeID: conn.GameData().EntityRuntimeID,
							Operation:       packet.MobEffectAdd,
							EffectType:      packet.EffectSpeed,
							Amplifier:       2,
							Particles:       false,
							Duration:        999999999,
						})
						sendMessage(conn, "§aSpeed has been turned on!")
					}
					continue
				case "jumpboost":
					if jumpboost {
						jumpboost = false
						_ = conn.WritePacket(&packet.MobEffect{
							EntityRuntimeID: conn.GameData().EntityRuntimeID,
							Operation:       packet.MobEffectRemove,
							EffectType:      packet.EffectJumpBoost,
							Amplifier:       2,
							Particles:       false,
							Duration:        1,
						})
						sendMessage(conn, "§aJumpBoost has been turned off!")
					} else {
						jumpboost = true
						_ = conn.WritePacket(&packet.MobEffect{
							EntityRuntimeID: conn.GameData().EntityRuntimeID,
							Operation:       packet.MobEffectAdd,
							EffectType:      packet.EffectJumpBoost,
							Amplifier:       2,
							Particles:       false,
							Duration:        999999999,
						})
						sendMessage(conn, "§aJumpBoost has been turned on!")
					}
					continue
				case "slowfalling":
					if slowfalling {
						slowfalling = false
						_ = conn.WritePacket(&packet.MobEffect{
							EntityRuntimeID: conn.GameData().EntityRuntimeID,
							Operation:       packet.MobEffectRemove,
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
							Flags:                   0 & packet.AdventureFlagNoClip,
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
							Flags:                   0 | packet.AdventureFlagNoClip,
							CommandPermissionLevel:  0,
							ActionPermissions:       0,
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
							Operation:       packet.MobEffectRemove,
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
				}
			default:
				break
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
			case *packet.AddPlayer:
				players[p.Username] = &Player{runtimeid: p.EntityRuntimeID, name: p.Username, metadata: p.EntityMetadata, dirtymetadata: p.EntityMetadata, uniqueid: p.EntityUniqueID}
				if v, ok := p.EntityMetadata[uint32(53)].(float32); ok {
					p.EntityMetadata[uint32(53)] = v + reach
				}
				if v, ok := p.EntityMetadata[uint32(54)].(float32); ok {
					p.EntityMetadata[uint32(54)] = v + reach
				}
				break
			case *packet.RemoveActor:
				for name, player := range players {
					if player.uniqueid == p.EntityUniqueID {
						delete(players, name)
						break
					}
				}
				break
			case *packet.SetActorData:
				for _, player := range players {
					if player.runtimeid == p.EntityRuntimeID {
						player.metadata = p.EntityMetadata
					}
				}
			case *packet.SetActorMotion:
				if p.EntityRuntimeID == conn.GameData().EntityRuntimeID {
					if antikb {
						continue
					}
				}
			case *packet.MoveActorAbsolute:
				pos := p.Position
				if killaura {
					go func() {
						_ = conn.WritePacket(&packet.InventoryTransaction{
							TransactionData: &protocol.UseItemOnEntityTransactionData{
								TargetEntityRuntimeID: p.EntityRuntimeID,
								ActionType:            protocol.UseItemOnEntityActionAttack,
								HotBarSlot:            0,
								HeldItem:              protocol.ItemInstance{},
								Position:              pos,
							},
						})
						time.Sleep(1 * time.Second)
					}()
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

func setReach(conn *minecraft.Conn, reach float32) {
	for _, p := range players {
		if v, ok := p.dirtymetadata[uint32(53)].(float32); ok {
			p.dirtymetadata[uint32(53)] = v + reach
		}
		if v, ok := p.dirtymetadata[uint32(54)].(float32); ok {
			p.dirtymetadata[uint32(54)] = v + reach
		}
		syncActor(conn, p.runtimeid, p.dirtymetadata)
	}
}

func syncActor(conn *minecraft.Conn, runtimeid uint64, metadata map[uint32]any) {
	_ = conn.WritePacket(&packet.SetActorData{EntityRuntimeID: runtimeid, EntityMetadata: metadata, Tick: 0})
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
	AuthInput struct {
		InputMode uint32
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
