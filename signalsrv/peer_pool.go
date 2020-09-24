package signalsrv

import (
	"log"

	"github.com/gorilla/websocket"
)

type PeerPool struct {
	connections      []*SignalingPeer
	servers          map[string][]*SignalingPeer
	addressSharing   bool
	maxAddressLength int
	appConfig        *AppConfig
}

func NewPeerPool(config *AppConfig) *PeerPool {
	return &PeerPool{
		connections:      make([]*SignalingPeer, 0),
		servers:          make(map[string][]*SignalingPeer),
		addressSharing:   config.AddressSharing,
		maxAddressLength: 256,
		appConfig:        config,
	}
}

func (pp *PeerPool) hasAddressSharing() bool {
	return pp.addressSharing
}

func (pp *PeerPool) add(conn *websocket.Conn) {
	pp.connections = append(pp.connections, NewSignalingPeer(pp, conn))
}

func (pp *PeerPool) getServerConnection(address string) []*SignalingPeer {
	if v, ok := pp.servers[address]; ok {
		return v
	}
	return nil
}

func (pp *PeerPool) isAddressAvailable(address string) bool {
	_, ok := pp.servers[address]
	if len(address) <= pp.maxAddressLength && (ok || pp.addressSharing) {
		return true
	}
	return false
}

func (pp *PeerPool) addServer(sp *SignalingPeer, address string) {
	if _, ok := pp.servers[address]; !ok {
		pp.servers[address] = make([]*SignalingPeer, 0)
	}
	pp.servers[address] = append(pp.servers[address], sp)
}

func (pp *PeerPool) removeServer(sp *SignalingPeer, address string) {
	servers, ok := pp.servers[address]
	if !ok {
		return
	}

	for i := 0; i < len(servers); i++ {
		if servers[i].connInfo != sp.connInfo {
			continue
		}
		pp.servers[address] = append(pp.servers[address][0:i], pp.servers[address][i+1:]...)
		break
	}

	if len(pp.servers[address]) == 0 {
		delete(pp.servers, address)
		log.Printf("Address %s released.", address)
	}
}

func (pp *PeerPool) removeConnection(sp *SignalingPeer) {
	for i := 0; i < len(pp.connections); i++ {
		if pp.connections[i].connInfo != sp.connInfo {
			continue
		}
		pp.connections = append(pp.connections[0:i], pp.connections[i+1:]...)
		break
	}
}

func (pp *PeerPool) count() int {
	return len(pp.connections)
}
