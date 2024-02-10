package dispatch

import (
	"errors"
	"fmt"
	"log"

	"github.com/Tnze/go-mc/chat"
	"github.com/Tnze/go-mc/data/packetid"
	"github.com/Tnze/go-mc/net"
	pk "github.com/Tnze/go-mc/net/packet"
	"github.com/Tnze/go-mc/server"
	"github.com/Tnze/go-mc/yggdrasil/user"
	"github.com/google/uuid"
)

type FakeMcServer struct {
	mcServer *server.Server
	kicker   *AlwaysKickLoginHandler
	pingInfo *FakePingInfo
	port     int
	listener *net.Listener
}

type AlwaysKickLoginHandler struct {
	kickMsg string
	onLogin func()
}

func NewAlwaysKickLoginHandler(kickMsg string) *AlwaysKickLoginHandler {
	return &AlwaysKickLoginHandler{
		kickMsg: kickMsg,
		onLogin: func() {},
	}
}

func (c *AlwaysKickLoginHandler) OnLogin(f func()) {
	c.onLogin = f
}

func (c *AlwaysKickLoginHandler) SetKickMsg(msg string) {
	c.kickMsg = msg
}

type EmptyGamePlay struct {
}

type LoginFailErr struct {
	reason chat.Message
}

func (e LoginFailErr) Error() string {
	return e.reason.String()
}

func (c *AlwaysKickLoginHandler) AcceptLogin(conn *net.Conn, protocol int32) (name string, id uuid.UUID, profilePubKey *user.PublicKey, properties []user.Property, err error) {
	// direct write kick packet
	_ = conn.WritePacket(pk.Marshal(
		packetid.ClientboundLoginDisconnect,
		chat.Text(c.kickMsg),
	))
	err = errors.New("Autokick")

	c.onLogin()

	return
}

type FakePingInfo struct {
	name        string
	description chat.Message
}

func (f *FakePingInfo) Name() string {
	return f.name
}

func (f *FakePingInfo) Protocol(clientProtocol int32) int {
	return int(clientProtocol)
}

func (f *FakePingInfo) FavIcon() string {
	return ""
}

func (f *FakePingInfo) Description() *chat.Message {
	return &f.description
}

func (f *FakePingInfo) SetDescription(description string) {
	f.description = chat.Text(description)
}

func NewFakePingInfo(name string, description string) *FakePingInfo {
	return &FakePingInfo{
		name:        name,
		description: chat.Text(description),
	}
}

func NewFakeMcServer(name string, motd string, kickMsg string, port int) *FakeMcServer {
	playerList := server.NewPlayerList(-1)
	serverInfo := NewFakePingInfo(name, motd)

	kicker := NewAlwaysKickLoginHandler(kickMsg)

	logger := log.Default()
	logger.SetFlags(log.LstdFlags | log.Lshortfile)

	s := server.Server{
		Logger: logger,
		ListPingHandler: struct {
			*server.PlayerList
			*FakePingInfo
		}{
			playerList,
			serverInfo,
		},
		LoginHandler: kicker,
	}

	return &FakeMcServer{
		mcServer: &s,
		kicker:   kicker,
		port:     port,
		listener: nil,
		pingInfo: serverInfo,
	}
}

func (s *FakeMcServer) Start() error {
	listener, err := net.ListenMC(fmt.Sprintf(":%d", s.port))
	if err != nil {
		return err
	}
	s.listener = listener
	for {
		conn, err := listener.Accept()
		if err != nil {
			return err
		}
		go s.mcServer.AcceptConn(&conn)
	}
}

func (s *FakeMcServer) Stop() error {
	if s.listener != nil {
		return s.listener.Close()
	}
	return nil
}

func (s *FakeMcServer) SetKickMsg(msg string) {
	s.kicker.SetKickMsg(msg)
}

func (s *FakeMcServer) SetMotd(motd string) {
	s.pingInfo.SetDescription(motd)
}

func (s *FakeMcServer) OnLogin(f func()) {
	s.kicker.OnLogin(f)
}
