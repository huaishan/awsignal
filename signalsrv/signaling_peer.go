package signalsrv

import (
	"fmt"
	"log"
	"time"

	"github.com/gorilla/websocket"
)

const (
	SignalingConnectionStateUninitialized = iota
	SignalingConnectionStateConnecting
	SignalingConnectionStateConnected
	SignalingConnectionStateDisconnection
	SignalingConnectionStateDisconnected
)

type SignalingPeer struct {
	state                    int
	connections              map[int16]*SignalingPeer
	nextIncomingConnectionId *ConnectionId
	connInfo                 string
	connectionPool           *PeerPool
	socket                   *websocket.Conn
	isAlive                  bool
	serverAddress            *string
	send                     chan *NetworkEvent
}

func NewSignalingPeer(pool *PeerPool, conn *websocket.Conn) *SignalingPeer {
	sp := &SignalingPeer{
		state:                    SignalingConnectionStateConnecting,
		connections:              make(map[int16]*SignalingPeer),
		nextIncomingConnectionId: NewConnectionId(16384),
		connInfo:                 conn.RemoteAddr().String(),
		connectionPool:           pool,
		socket:                   conn,
		isAlive:                  true,
		serverAddress:            nil,
		send:                     make(chan *NetworkEvent, 256),
	}
	sp.run()
	log.Printf("[%s] connected on %s", sp.connInfo, sp.socket.LocalAddr().String())
	sp.state = SignalingConnectionStateConnected

	return sp
}

func (sp *SignalingPeer) GetName() string {
	return fmt.Sprintf("[%s]", sp.connInfo)
}

func (sp *SignalingPeer) run() {
	go sp.readPump()
	go sp.writePump()
}

func (sp *SignalingPeer) sendToClient(evt *NetworkEvent) {
	if sp.state != SignalingConnectionStateConnected {
		return
	}
	sp.send <- evt
}

func (sp *SignalingPeer) Cleanup() {
	if sp.state == SignalingConnectionStateDisconnection || sp.state == SignalingConnectionStateDisconnected {
		return
	}

	sp.state = SignalingConnectionStateDisconnection
	log.Println(sp.GetName(), " disconnection.")
	sp.connectionPool.removeConnection(sp)

	// disconnect all connections
	for k := range sp.connections {
		sp.disconnect(NewConnectionId(k))
	}

	// make sure the server address is freed
	if sp.serverAddress != nil {
		sp.stopServer()
	}

	sp.socket.Close()

	log.Println(sp.GetName(), "removed", sp.connectionPool.count(), "connections left.")
	sp.state = SignalingConnectionStateDisconnected
}

func (sp *SignalingPeer) handleIncomingEvent(evt *NetworkEvent) {
	switch evt.Type {
	case NetEventTypeNewConnection:
		addr := evt.GetInfo().StringData
		if addr != nil {
			sp.connect(*addr, evt.ConnectionId)
		}
	case NetEventTypeConnectionFailed:
	case NetEventTypeDisconnected:
		sp.disconnect(evt.ConnectionId)
	case NetEventTypeServerInitialized:
		addr := evt.GetInfo().StringData
		if addr != nil {
			sp.startServer(*addr)
		}
	case NetEventTypeServerInitFailed:
	case NetEventTypeServerClosed:
		sp.stopServer()
	case NetEventTypeReliableMessageReceived:
		sp.sendData(evt.ConnectionId, evt.GetMessageData(), true)
	case NetEventTypeUnreliableMessageReceived:
		sp.sendData(evt.ConnectionId, evt.GetMessageData(), false)
	}
}

func (sp *SignalingPeer) internalAddIncomingPeer(peer *SignalingPeer) {
	id := sp.nextConnectionId()
	sp.connections[id.ID] = peer
	sp.sendToClient(NewNetworkEvent(NetEventTypeNewConnection, id, &NetEventData{Type: NetEventDataTypeNull}))
}

func (sp *SignalingPeer) internalAddOutgoingPeer(peer *SignalingPeer, id *ConnectionId) {
	sp.connections[id.ID] = peer
	sp.sendToClient(NewNetworkEvent(NetEventTypeNewConnection, id, &NetEventData{Type: NetEventDataTypeNull}))
}

func (sp *SignalingPeer) internalRemovePeer(id *ConnectionId) {
	delete(sp.connections, id.ID)
	sp.sendToClient(NewNetworkEvent(NetEventTypeDisconnected, id, &NetEventData{Type: NetEventDataTypeNull}))
}

func (sp *SignalingPeer) findPeerConnectionId(otherPeer *SignalingPeer) *ConnectionId {
	for k, v := range sp.connections {
		if v.connInfo != otherPeer.connInfo {
			continue
		}
		return NewConnectionId(k)
	}
	return nil
}

func (sp *SignalingPeer) nextConnectionId() *ConnectionId {
	result := sp.nextIncomingConnectionId
	sp.nextIncomingConnectionId = NewConnectionId(sp.nextIncomingConnectionId.ID + 1)
	return result
}

func (sp *SignalingPeer) connect(address string, id *ConnectionId) {
	sc := sp.connectionPool.getServerConnection(address)
	if sc != nil && len(sc) == 1 {
		sc[0].internalAddIncomingPeer(sp)
		sp.internalAddOutgoingPeer(sc[0], id)
	} else {
		sp.sendToClient(NewNetworkEvent(NetEventTypeConnectionFailed, id, &NetEventData{Type: NetEventDataTypeNull}))
	}
}

func (sp *SignalingPeer) connectJoin(address string) {
	sc := sp.connectionPool.getServerConnection(address)
	if sc != nil {
		for _, v := range sc {
			if v.connInfo == sp.connInfo {
				continue
			}
			v.internalAddIncomingPeer(sp)
			sp.internalAddIncomingPeer(v)
		}
	}
}

func (sp *SignalingPeer) disconnect(id *ConnectionId) {
	otherPeer := sp.connections[id.ID]
	if otherPeer != nil {
		idOfOther := otherPeer.findPeerConnectionId(sp)
		sp.internalRemovePeer(id)
		otherPeer.internalRemovePeer(idOfOther)
	}
}

func (sp *SignalingPeer) startServer(address string) {
	if sp.serverAddress != nil {
		sp.stopServer()
	}
	if sp.connectionPool.isAddressAvailable(address) {
		sp.serverAddress = &address
		sp.connectionPool.addServer(sp, address)
		sp.sendToClient(NewNetworkEvent(
			NetEventTypeServerInitialized,
			INVALIDConnectionId,
			&NetEventData{Type: NetEventDataTypeUTF16String, StringData: &address},
		))
		if sp.connectionPool.hasAddressSharing() {
			sp.connectJoin(address)
		}
	} else {
		sp.sendToClient(NewNetworkEvent(
			NetEventTypeServerInitFailed,
			INVALIDConnectionId,
			&NetEventData{Type: NetEventDataTypeUTF16String, StringData: &address},
		))
	}
}

func (sp *SignalingPeer) stopServer() {
	if sp.serverAddress == nil {
		return
	}
	sp.connectionPool.removeServer(sp, *sp.serverAddress)
	sp.sendToClient(NewNetworkEvent(NetEventTypeServerClosed, INVALIDConnectionId, &NetEventData{Type: NetEventDataTypeNull}))
	sp.serverAddress = nil
}

func (sp *SignalingPeer) forwardMessage(senderPeer *SignalingPeer, msg *NetEventData, reliable bool) {
	id := sp.findPeerConnectionId(senderPeer)
	if reliable {
		sp.sendToClient(NewNetworkEvent(NetEventTypeReliableMessageReceived, id, msg))
	} else {
		sp.sendToClient(NewNetworkEvent(NetEventTypeUnreliableMessageReceived, id, msg))
	}
}

func (sp *SignalingPeer) sendData(id *ConnectionId, msg *NetEventData, reliable bool) {
	if peer, ok := sp.connections[id.ID]; ok {
		peer.forwardMessage(sp, msg, reliable)
	}

}

const (
	writeWait      = 5 * time.Second
	pongWait       = 5 * time.Second
	pingPeriod     = 3 * time.Second
	maxMessageSize = 1048576
)

func (sp *SignalingPeer) readPump() {
	defer func() {
		sp.Cleanup()
	}()
	sp.socket.SetReadLimit(maxMessageSize)
	sp.socket.SetReadDeadline(time.Now().Add(pongWait))
	sp.socket.SetPongHandler(func(string) error { sp.socket.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		_, msg, err := sp.socket.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseNoStatusReceived) {
				log.Println(sp.GetName(), "CLOSED!")
			} else if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			} else {
				log.Println(err)
			}
			return
		}
		evt, err := FromByteArray(msg)
		log.Println(sp.GetName(), "INC: ", evt.String())
		sp.handleIncomingEvent(evt)
	}
}

func (sp *SignalingPeer) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		sp.Cleanup()
	}()
	for {
		select {
		case <-ticker.C:
			sp.socket.SetWriteDeadline(time.Now().Add(writeWait))
			if err := sp.socket.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		case evt := <-sp.send:
			log.Printf("%s OUT: %s", sp.GetName(), evt.String())
			sp.socket.WriteMessage(websocket.BinaryMessage, evt.ToByteArray())
		}
	}
}
