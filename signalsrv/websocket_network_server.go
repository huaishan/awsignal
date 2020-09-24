package signalsrv

import (
	"github.com/gorilla/websocket"
)

type WebsocketNetworkServer struct {
	pool map[string]*PeerPool
}

func NewWebsocketNetworkServer() *WebsocketNetworkServer {
	return &WebsocketNetworkServer{
		pool: make(map[string]*PeerPool),
	}
}

func (wns *WebsocketNetworkServer) OnConnection(socket *websocket.Conn, config *AppConfig) {
	if _, ok := wns.pool[config.AppName]; !ok {
		wns.pool[config.AppName] = NewPeerPool(config)
	}
	wns.pool[config.AppName].add(socket)
}
