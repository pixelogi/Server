package manager

import (
	"crypto/rsa"
	"fmt"
	"log"
	"sync"

	"github.com/gorilla/websocket"
	"golang.org/x/crypto/bcrypt"
)

type (
	ManagerState  uint8
	GRPCPeerState uint8
	WSState       uint8
	SquadType     string
	SquadEvent    string

	GRPCPeer struct {
		Conn  GrpcManager_LinkServer
		State GRPCPeerState
	}

	WSPeer struct {
		Conn  *websocket.Conn
		State WSState
	}

	Manager struct {
		State     ManagerState
		GRPCPeers map[string]*GRPCPeer
		WSPeers   map[string]*WSPeer
		Squads    map[string]*Squad
		SquadDBManager *SquadDBManager
		HostedSquadDBManager *HostedSquadDBManager
		PeerDBManager *PeerDBManager
		*sync.RWMutex
	}
)

const (
	PRIVATE SquadType = "private"
	PUBLIC  SquadType = "public"
)

const (
	INCOMING_MEMBER        SquadEvent = "incoming_member"
	HOSTED_INCOMING_MEMBER SquadEvent = "hosted_incoming_member"
	LEAVING_MEMBER         SquadEvent = "leaving_member"
	HOSTED_LEAVING_MEMBER  SquadEvent = "hosted_leaving_member"
)

const (
	ON ManagerState = iota
	OFF
)

const (
	CONNECTED GRPCPeerState = iota
	SLEEP
)

const DB_NAME string = "zippytal_server"

func NewManager() (manager *Manager, err error) {
	hostedSquadDBManagerCh, hostedSquadErrCh := NewHostedSquadDBManager("localhost", 27017)
	squadDBManagerCh, squadErrCh := NewSquadDBManager("localhost", 27017)
	peerDBManagerCh, peerErrCh := NewPeerDBManager("localhost", 27017)
	manager = &Manager{
			State:     ON,
			GRPCPeers: make(map[string]*GRPCPeer),
			WSPeers:   make(map[string]*WSPeer),
			Squads:    make(map[string]*Squad),
			RWMutex:   &sync.RWMutex{},
	}
	assignmemnt: for i := 0; i < 2; i++ {
		select {
			case h := <- hostedSquadDBManagerCh:
				manager.HostedSquadDBManager = h
			case s := <-squadDBManagerCh:
				manager.SquadDBManager = s
			case p := <-peerDBManagerCh:
				manager.PeerDBManager = p
			case eh := <-hostedSquadErrCh:
				err = eh
				break assignmemnt
			case es := <-squadErrCh:
				err = es
				break assignmemnt
			case ep := <-peerErrCh:
				err = ep
				break assignmemnt
		}
	}
	return
}

func (manager *Manager) CreateSquad(id string, owner string, name string, squadType SquadType, password string, squadNetworkType SquadNetworkType, host string) (err error) {
	squadPass := ""
	if squadType == PRIVATE {
		if output, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost); err != nil {
			return err
		} else {
			squadPass = string(output)
		}
	} else {
		_ = password
	}
	squad := Squad{
		Owner:             owner,
		Name:              name,
		NetworkType:       squadNetworkType,
		HostId:            host,
		ID:                id,
		SquadType:         squadType,
		Password:          squadPass,
		Members:           make([]string, 0),
		AuthorizedMembers: make([]*rsa.PublicKey, 0),
		RWMutex:           new(sync.RWMutex),
	}
	manager.Squads[id] = &squad
	
	if err != nil {
		return
	}
	return
}

func (manager *Manager) DeleteSquad(id string, from string) (err error) {
	if _, ok := manager.Squads[id]; !ok {
		err = fmt.Errorf("this squad does not exist")
		return
	}
	delete(manager.Squads, id)
	return
}

func (manager *Manager) ModifySquad(id string, from string, name string, squadType SquadType, password string) (err error) {
	if _, ok := manager.Squads[id]; !ok {
		err = fmt.Errorf("this squad does not exist")
		return
	}
	if manager.Squads[id].Owner != from {
		err = fmt.Errorf("you are not the owner of this squad so you can't modifiy it")
		return
	}
	manager.Squads[id].Lock()
	defer manager.Squads[id].Unlock()
	manager.Squads[id].Name = name
	squadPass := ""
	if squadType == PRIVATE {
		output, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		squadPass = string(output)
		manager.Squads[id].Password = squadPass
		manager.Squads[id].SquadType = PRIVATE
	} else if squadType == PUBLIC {
		manager.Squads[id].Password = squadPass
		manager.Squads[id].SquadType = PRIVATE
	}
	return
}

func (manager *Manager) ConnectToSquad(id string, from string, password string, grpc bool) (err error) {
	if _, ok := manager.Squads[id]; !ok {
		err = fmt.Errorf("this squad does not exist")
		return
	}
	var INCOMING string
	if manager.Squads[id].NetworkType == MESH {
		INCOMING = string(INCOMING_MEMBER)
	} else {
		INCOMING = string(HOSTED_INCOMING_MEMBER)
	}
	if manager.Squads[id].SquadType == PUBLIC {
		manager.Squads[id].Join(from)
		for _, member := range manager.Squads[id].Members {
			if member != from {
				if _, ok := manager.GRPCPeers[member]; ok {
					if err := manager.GRPCPeers[member].Conn.Send(&Response{
						Type:    INCOMING,
						Success: true,
						Payload: map[string]string{
							"id": from,
						},
					}); err != nil {
						delete(manager.GRPCPeers, member)
						return err
					}
				} else if _, ok := manager.WSPeers[member]; ok {
					if err = manager.WSPeers[member].Conn.WriteJSON(map[string]interface{}{
						"from":    from,
						"to":      member,
						"type":    INCOMING,
						"payload": map[string]string{},
					}); err != nil {
						log.Println(err)
						return
					}
				}
			}
		}
		return
	}
	if manager.Squads[id].SquadType == PRIVATE {
		accessGranted := manager.Squads[id].Authenticate(password)
		if !accessGranted {
			err = fmt.Errorf("access denied : wrong password")
			return
		}
		manager.Squads[id].Join(from)
		for _, member := range manager.Squads[id].Members {
			if member != from {
				if _, ok := manager.GRPCPeers[member]; ok {
					if err := manager.GRPCPeers[member].Conn.Send(&Response{
						Type:    INCOMING,
						Success: true,
						Payload: map[string]string{
							"id": from,
						},
					}); err != nil {
						delete(manager.GRPCPeers, member)
						return err
					}
				} else if _, ok := manager.WSPeers[member]; ok {
					if err = manager.WSPeers[member].Conn.WriteJSON(map[string]interface{}{
						"from":    from,
						"to":      member,
						"type":    INCOMING,
						"payload": map[string]string{},
					}); err != nil {
						log.Println(err)
						return
					}
				}
			}
		}
		return
	}
	err = fmt.Errorf("squad type is undetermined")
	return
}

func (manager *Manager) LeaveSquad(id string, from string, grpc bool) (err error) {
	if _, ok := manager.Squads[id]; !ok {
		err = fmt.Errorf("this squad does not exist")
		return
	}
	squad := manager.Squads[id]
	var memberIndex int
	for i, member := range squad.Members {
		if member == from {
			memberIndex = i
			break
		}
	}
	squad.Lock()
	squad.Members[len(squad.Members)-1], squad.Members[memberIndex] = squad.Members[memberIndex], squad.Members[len(squad.Members)-1]
	newMembers := squad.Members[:len(squad.Members)-1]
	squad.Members = newMembers
	squad.Unlock()
	manager.RLock()
	var LEAVING string
	if squad.NetworkType == MESH {
		LEAVING = string(LEAVING_MEMBER)
	} else {
		LEAVING = string(HOSTED_LEAVING_MEMBER)
	}
	defer manager.RUnlock()
	for _, member := range manager.Squads[id].Members {
		if member != from {
			if _, ok := manager.GRPCPeers[member]; ok {
				if err := manager.GRPCPeers[member].Conn.Send(&Response{
					Type:    LEAVING,
					Success: true,
					Payload: map[string]string{
						"id": from,
					},
				}); err != nil {
					delete(manager.GRPCPeers, member)
					return err
				}
			} else if _, ok := manager.WSPeers[member]; ok {
				if err = manager.WSPeers[member].Conn.WriteJSON(map[string]interface{}{
					"from":    from,
					"to":      member,
					"type":    LEAVING,
					"payload": map[string]string{},
				}); err != nil {
					log.Println(err)
					return
				}
			}
		}
	}
	return
}

func (manager *Manager) AddGrpcPeer(peer GrpcManager_LinkServer, id string, req *Request) (err error) {
	fmt.Printf("adding peer %s\n", req.From)
	manager.Lock()
	manager.GRPCPeers[req.From] = &GRPCPeer{Conn: peer, State: CONNECTED}
	manager.Unlock()
	if _, ok := req.Payload["to"]; ok {
		if _, ok := manager.GRPCPeers[req.From]; ok {
			if err = manager.GRPCPeers[req.From].Conn.Send(&Response{
				Type:    req.Type,
				Success: true,
				Payload: req.Payload,
			}); err != nil {
				return
			}
		}
	}
	err = manager.manage(peer)
	delete(manager.GRPCPeers, req.From)
	return
}

func (manager *Manager) manage(peer GrpcManager_LinkServer) (err error) {
	done, errch := make(chan struct{}), make(chan error)
	go func() {
		for {
			req, err := peer.Recv()
			if err != nil {
				errch <- err
				return
			}
			fmt.Println(req)
			if _, ok := req.Payload["to"]; ok {
				to := req.Payload["to"]
				if _, ok := manager.GRPCPeers[to]; ok {
					if err := manager.GRPCPeers[to].Conn.Send(&Response{
						Type:    req.Type,
						Success: true,
						Payload: req.Payload,
					}); err != nil {
						errch <- err
						return
					}
				} else if _, ok := manager.WSPeers[to]; ok {
					if err = manager.WSPeers[to].Conn.WriteJSON(map[string]interface{}{
						"from":    req.From,
						"to":      to,
						"type":    req.Type,
						"payload": req.Payload,
					}); err != nil {
						log.Println(err)
						return
					}
				}
			}
		}
	}()
	select {
	case <-done:
		log.Println("manage is done")
		return
	case err = <-errch:
		return
	case <-peer.Context().Done():
		err = peer.Context().Err()
		return
	}
}
