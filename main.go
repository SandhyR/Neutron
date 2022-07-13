package main

import (
	"bytes"
	"errors"
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/auth"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"golang.org/x/oauth2"
	"io"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

var players = map[string]*Player{}
var proxy = Proxy{reach: 0.0, fly: false, antikb: false, jumpboost: false, speed: false, killaura: false, haste: false, slowfalling: false, noclip: false, nightvision: false}
var application App
var MessagePrefix = "§o§l§6Neutron§r§7 > "
var PREFIX = "/."

type Player struct {
	name          string
	runtimeid     uint64
	uniqueid      int64
	dirtymetadata map[uint32]any
	metadata      map[uint32]any
}

type App struct {
	label *widget.Label
}

type Writter struct {
	w io.Writer
}

type Proxy struct {
	listener    *minecraft.Listener
	serverConn  *minecraft.Conn
	reach       float32
	fly         bool
	antikb      bool
	jumpboost   bool
	speed       bool
	killaura    bool
	haste       bool
	slowfalling bool
	noclip      bool
	nightvision bool
}

func (Writter) Write(p []byte) (n int, err error) {
	str := string(p[:])
	application.label.SetText(str)
	application.label.Refresh()
	return len(p), nil
}

func main() {
	a := app.New()
	w := a.NewWindow("Neutron")
	w.Resize(fyne.NewSize(400, 400))
	entryip, entryport, entrylocalport, entryinput, entrymodel := widget.NewEntry(), widget.NewEntry(), widget.NewEntry(), widget.NewSelectEntry([]string{"Mouse & Keyboard", "Touch", "Controller"}), widget.NewEntry()
	label := widget.NewLabel("")
	form := widget.NewForm(widget.NewFormItem("Local Port", entrylocalport), widget.NewFormItem("Target IP", entryip), widget.NewFormItem("Target Port", entryport), widget.NewFormItem("Input", entryinput), widget.NewFormItem("Device Model", entrymodel))

	form.OnSubmit = func() {
		//skidded from tal
		if !loopbackExempted() {
			const loopbackExemptCmd = `CheckNetIsolation LoopbackExempt -a -n="Microsoft.MinecraftUWP_8wekyb3d8bbwe"`
			label.SetText(fmt.Sprintf("You are currently unable to join the proxy on this machine. Run %v in an admin PowerShell session to be able to.\n", loopbackExemptCmd))
			label.Refresh()
		}
		startProxy(entrylocalport.Text, entryip.Text, entryport.Text, ToInput(entryinput.Text), entrymodel.Text)
	}

	form.OnCancel = func() {
		stopProxy()
	}
	form.SubmitText = "Start"
	form.CancelText = "Stop"

	w.SetContent(container.NewVBox(widget.NewLabelWithStyle("Neutron", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}), form, label))
	application = App{label: label}
	w.ShowAndRun()
}

func stopProxy() {
	if proxy.serverConn != nil {
		proxy.serverConn.Close()
	}
	if proxy.listener != nil {
		proxy.listener.Close()
	}
	application.label.SetText("Successfully stopped the proxy")
	application.label.Refresh()
}

func startProxy(localport string, ip string, port string, input int, devicemodel string) {
	token, err := auth.RequestLiveTokenWriter(Writter{})
	if err != nil {
		application.label.SetText("Session expired")
		application.label.Refresh()
	}
	src := auth.RefreshTokenSource(token)

	p, err := minecraft.NewForeignStatusProvider(ip + ":" + port)
	if err != nil {
		panic(err)
	}
	proxy.listener, err = minecraft.ListenConfig{
		StatusProvider: p,
	}.Listen("raknet", "0.0.0.0:"+localport)
	if err != nil {
		panic(err)
	}
	defer proxy.listener.Close()
	for {
		c, err := proxy.listener.Accept()
		if err != nil {
			panic(err)
		}
		go handleConn(c.(*minecraft.Conn), src, ip, port, input, devicemodel)
	}
}

// handleConn handles a new incoming minecraft.Conn from the minecraft.Listener passed.
func handleConn(conn *minecraft.Conn, src oauth2.TokenSource, ip string, port string, input int, devicemodel string) {
	clientdata := conn.ClientData()
	clientdata.CurrentInputMode = input
	clientdata.DeviceModel = devicemodel
	var err error
	proxy.serverConn, err = minecraft.Dialer{
		TokenSource: src,
		ClientData:  clientdata,
	}.Dial("raknet", ip+":"+port)
	if err != nil {
		panic(err)
	}
	var g sync.WaitGroup
	g.Add(2)
	go func() {
		if err := conn.StartGame(proxy.serverConn.GameData()); err != nil {
			panic(err)
		}
		g.Done()
	}()
	go func() {
		if err := proxy.serverConn.DoSpawn(); err != nil {
			panic(err)
		}
		g.Done()
	}()
	g.Wait()

	go func() {
		// serverbound (client -> server)
		defer proxy.listener.Disconnect(conn, "connection lost")
		defer proxy.serverConn.Close()
		for {
			pk, err := conn.ReadPacket()
			if err != nil {
				return
			}

			switch p := pk.(type) {
			case *packet.PlayerAuthInput:
				p.InputMode = uint32(input)
				break
			case *packet.RequestAbility:
				//TODO: use this so that flying is not detected
				//https://github.com/pmmp/PocketMine-MP/blob/4ec97d0f7ae84270abc77f02fc57b4f60d1ba87d/src/network/mcpe/handler/InGamePacketHandler.php#L974
				if p.Ability == protocol.AbilityFlying {
					if p.Value == proxy.fly {
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
					if proxy.fly {
						proxy.fly = false
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
						proxy.fly = true
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
					if proxy.antikb {
						proxy.antikb = false
						sendMessage(conn, "§aAnti Knockback has been turned off!")
					} else {
						proxy.antikb = true
						sendMessage(conn, "§aAnti Knockback has been turned on!")
					}
					continue
				case "killaura":
					if proxy.killaura {
						proxy.killaura = false
						sendMessage(conn, "§aKill Aura has been turned off!")
					} else {
						proxy.killaura = true
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
						proxy.reach = float32(nreach)
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
					if proxy.haste {
						proxy.haste = false
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
						proxy.haste = true
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
					if proxy.speed {
						proxy.speed = false
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
						proxy.speed = true
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
					if proxy.jumpboost {
						proxy.jumpboost = false
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
						proxy.jumpboost = true
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
					if proxy.slowfalling {
						proxy.slowfalling = false
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
						proxy.slowfalling = true
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
					if proxy.noclip {
						proxy.noclip = false
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
						proxy.noclip = true
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
					if proxy.nightvision {
						proxy.nightvision = false
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
						proxy.nightvision = true
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
			if err := proxy.serverConn.WritePacket(pk); err != nil {
				if disconnect, ok := errors.Unwrap(err).(minecraft.DisconnectError); ok {
					_ = proxy.listener.Disconnect(conn, disconnect.Error())
				}
				return
			}
		}
	}()
	go func() {
		// clientbound (server -> client)
		defer proxy.serverConn.Close()
		defer proxy.listener.Disconnect(conn, "connection lost")
		for {
			pk, err := proxy.serverConn.ReadPacket()
			if err != nil {
				if disconnect, ok := errors.Unwrap(err).(minecraft.DisconnectError); ok {
					_ = proxy.listener.Disconnect(conn, disconnect.Error())
				}
				return
			}
			switch p := pk.(type) {
			case *packet.AddPlayer:
				players[p.Username] = &Player{runtimeid: p.EntityRuntimeID, name: p.Username, metadata: p.EntityMetadata, dirtymetadata: p.EntityMetadata, uniqueid: p.EntityUniqueID}
				if v, ok := p.EntityMetadata[uint32(53)].(float32); ok {
					p.EntityMetadata[uint32(53)] = v + proxy.reach
				}
				if v, ok := p.EntityMetadata[uint32(54)].(float32); ok {
					p.EntityMetadata[uint32(54)] = v + proxy.reach
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
					if proxy.antikb {
						continue
					}
				}
			case *packet.MoveActorAbsolute:
				pos := p.Position
				if proxy.killaura {
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

func loopbackExempted() bool {
	if runtime.GOOS != "windows" {
		return true
	}
	data, _ := exec.Command("CheckNetIsolation", "LoopbackExempt", "-s", `-n="microsoft.minecraftuwp_8wekyb3d8bbwe"`).CombinedOutput()
	return bytes.Contains(data, []byte("microsoft.minecraftuwp_8wekyb3d8bbwe"))
}

func ToInput(input string) int {
	switch input {
	case "Mouse & Keyboard":
		return 1
	case "Touch":
		return 2
	case "Controller":
		return 3

	}
	return 0
}
