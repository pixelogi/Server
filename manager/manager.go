package manager

import (
	"context"
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
		State                ManagerState
		GRPCPeers            map[string]*GRPCPeer
		WSPeers              map[string]*WSPeer
		Squads               map[string]*Squad
		SquadDBManager       *SquadDBManager
		HostedSquadDBManager *HostedSquadDBManager
		PeerDBManager        *PeerDBManager
		AuthManager          *AuthManager
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
	hostedSquadDBManager, err := NewHostedSquadDBManager("localhost", 27017)
	if err != nil {
		return
	}
	squadDBManager, err := NewSquadDBManager("localhost", 27017)
	if err != nil {
		return
	}
	peerDBManager, err := NewPeerDBManager("localhost", 27017)
	if err != nil {
		return
	}
	manager = &Manager{
		State:                ON,
		GRPCPeers:            make(map[string]*GRPCPeer),
		WSPeers:              make(map[string]*WSPeer),
		Squads:               make(map[string]*Squad),
		SquadDBManager:       squadDBManager,
		HostedSquadDBManager: hostedSquadDBManager,
		PeerDBManager:        peerDBManager,
		RWMutex:              &sync.RWMutex{},
		AuthManager:          NewAuthManager(),
	}
	return
}

func (manager *Manager) CreatePeer(peerId string, peerKey string, peerUsername string) (err error) {
	peer := &Peer{
		PubKey: peerKey,
		Id:     peerId,
		Name:   peerUsername,
	}
	err = manager.PeerDBManager.AddNewPeer(context.Background(), peer)
	return
}

func (manager *Manager) PeerAuthInit(peerId string) (encryptedToken []byte, err error) {
	if _, ok := manager.AuthManager.AuthTokenPending[peerId]; ok {
		err = fmt.Errorf("user in authentification")
		return
	}
	peer, err := manager.PeerDBManager.GetPeer(context.Background(), peerId)
	if err != nil {
		delete(manager.AuthManager.AuthTokenPending, peerId)
		return
	}
	encryptedToken, err = manager.AuthManager.GenerateAuthToken(peer.Id, peer.PubKey)
	if err != nil {
		delete(manager.AuthManager.AuthTokenPending, peerId)
	}
	return
}

func (manager *Manager) PeerAuthVerif(peerId string, token []byte) (err error) {
	if _, ok := manager.AuthManager.AuthTokenPending[peerId]; !ok {
		err = fmt.Errorf("the peer %s have not initiated auth", peerId)
		return
	}
	if manager.AuthManager.AuthTokenPending[peerId] != string(token) {
		err = fmt.Errorf("authentification failed wrong key")
	} else {
		manager.AuthManager.AuthTokenValid[string(token)] = peerId
	}
	fmt.Println("done")
	delete(manager.AuthManager.AuthTokenPending, peerId)
	return
}

func (manager *Manager) GetSquadSByOwner(token string, owner string, lastIndex int64) (squad []*Squad, err error) {
	if _, ok := manager.AuthManager.AuthTokenValid[token]; !ok {
		err = fmt.Errorf("not a valid token provided")
		return
	}
	if manager.AuthManager.AuthTokenValid[token] != owner {
		err = fmt.Errorf("invalid access")
		return
	}
	squad, err = manager.SquadDBManager.GetSquadsByOwner(context.Background(), owner, 100, lastIndex)
	return
}

func (manager *Manager) CreateSquad(token string, id string, owner string, name string, squadType SquadType, password string, squadNetworkType SquadNetworkType, host string) (err error) {
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
		AuthorizedMembers: make([]string, 0),
		mutex:             new(sync.RWMutex),
	}
	manager.Squads[id] = &squad
	switch squadNetworkType {
	case MESH:
		err = manager.SquadDBManager.AddNewSquad(context.Background(), &squad)
	case HOSTED:
		err = manager.HostedSquadDBManager.AddNewHostedSquad(context.Background(), &squad)
	}
	if err != nil {
		return
	}
	return
}

func (manager *Manager) DeleteSquad(token string, id string, from string) (err error) {
	if _, ok := manager.Squads[id]; !ok {
		err = fmt.Errorf("this squad does not exist")
		return
	}
	switch manager.Squads[id].NetworkType {
	case MESH:
		err = manager.SquadDBManager.DeleteSquad(context.Background(), id)
	case HOSTED:
		err = manager.HostedSquadDBManager.DeleteHostedSquad(context.Background(), id)
	}
	delete(manager.Squads, id)
	return
}

func (manager *Manager) ModifySquad(token string, id string, from string, name string, squadType SquadType, password string) (err error) {
	if _, ok := manager.Squads[id]; !ok {
		err = fmt.Errorf("this squad does not exist")
		return
	}
	if manager.Squads[id].Owner != from {
		err = fmt.Errorf("you are not the owner of this squad so you can't modifiy it")
		return
	}
	manager.Squads[id].mutex.Lock()
	defer manager.Squads[id].mutex.Unlock()
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

func (manager *Manager) ConnectToSquad(token string, id string, from string, password string, networkType SquadNetworkType) (err error) {
	var squad *Squad
	if networkType == MESH {
		if squad, err = manager.SquadDBManager.GetSquad(context.Background(), id); err != nil {
			return
		}
	} else if networkType == HOSTED {
		if squad, err = manager.HostedSquadDBManager.GetHostedSquad(context.Background(), id); err != nil {
			return
		}
	}
	fmt.Println(token)
	fmt.Println(from)
	fmt.Println(squad.AuthorizedMembers)
	fmt.Println(manager.AuthManager.AuthTokenValid[token])
	var contains bool = false
	if _, ok := manager.AuthManager.AuthTokenValid[token]; ok {
		if manager.AuthManager.AuthTokenValid[token] == from {
			for _, am := range squad.AuthorizedMembers {
				fmt.Println("authorized member", am)
				if am == from {
					contains = true
				}
			}
		}
	}
	fmt.Println(contains)
	var INCOMING string
	if squad.NetworkType == MESH {
		INCOMING = string(INCOMING_MEMBER)
	} else {
		INCOMING = string(HOSTED_INCOMING_MEMBER)
	}
	squad.mutex = &sync.RWMutex{}
	if squad.SquadType == PUBLIC || contains {
		squad.Join(from)
		for _, member := range squad.Members {
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
		switch networkType {
		case MESH:
			err = manager.SquadDBManager.UpdateSquadMembers(context.Background(), squad.ID, squad.Members)
		case HOSTED:
			err = manager.HostedSquadDBManager.UpdateHostedSquadMembers(context.Background(), squad.ID, squad.Members)
		}
		return
	}
	if squad.SquadType == PRIVATE {
		if !squad.Authenticate(password) {
			err = fmt.Errorf("access denied : wrong password")
			return
		}
		squad.Join(from)
		for _, member := range squad.Members {
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
		switch networkType {
		case MESH:
			err = manager.SquadDBManager.UpdateSquadMembers(context.Background(), squad.ID, squad.Members)
		case HOSTED:
			err = manager.HostedSquadDBManager.UpdateHostedSquadMembers(context.Background(), squad.ID, squad.Members)
		}
		return
	}
	err = fmt.Errorf("squad type is undetermined")
	return
}

func (manager *Manager) LeaveSquad(id string, from string, networkType SquadNetworkType) (err error) {
	var squad *Squad
	if networkType == MESH {
		if squad, err = manager.SquadDBManager.GetSquad(context.Background(), id); err != nil {
			return
		}
	} else if networkType == HOSTED {
		if squad, err = manager.HostedSquadDBManager.GetHostedSquad(context.Background(), id); err != nil {
			return
		}
	}
	squad.mutex = &sync.RWMutex{}
	var memberIndex int
	for i, member := range squad.Members {
		if member == from {
			memberIndex = i
			break
		}
	}
	squad.mutex.Lock()
	fmt.Println(squad.Members)
	if len(squad.Members) < 2 {
		newMembers := []string{}
		squad.Members = newMembers
	} else {
		squad.Members[len(squad.Members)-1], squad.Members[memberIndex] = squad.Members[memberIndex], squad.Members[len(squad.Members)-1]
		newMembers := squad.Members[:len(squad.Members)-1]
		squad.Members = newMembers
	}
	squad.mutex.Unlock()
	manager.RLock()
	var LEAVING string
	if squad.NetworkType == MESH {
		LEAVING = string(LEAVING_MEMBER)
	} else {
		LEAVING = string(HOSTED_LEAVING_MEMBER)
	}
	defer manager.RUnlock()
	for _, member := range squad.Members {
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
	fmt.Println(squad.Members)
	switch networkType {
	case MESH:
		err = manager.SquadDBManager.UpdateSquadMembers(context.Background(), squad.ID, squad.Members)
	case HOSTED:
		err = manager.HostedSquadDBManager.UpdateHostedSquadMembers(context.Background(), squad.ID, squad.Members)
	}
	return
}

func (manager *Manager) ListAllSquads(lastIndex int64, networkType SquadNetworkType) (squads []*Squad, err error) {
	switch networkType {
	case MESH:
		squads, err = manager.SquadDBManager.GetSquads(context.Background(), 100, lastIndex)
	case HOSTED:
		squads, err = manager.HostedSquadDBManager.GetHostedSquads(context.Background(), 100, lastIndex)
	}
	return
}

func (manager *Manager) ListSquadsByName(lastIndex int64, squadName string, networkType SquadNetworkType) (squads []*Squad, err error) {
	switch networkType {
	case MESH:
		squads, err = manager.SquadDBManager.GetSquadsByName(context.Background(), squadName, 100, lastIndex)
	case HOSTED:
		squads, err = manager.HostedSquadDBManager.GetHostedSquadsByName(context.Background(), squadName, 100, lastIndex)
	}
	return
}

func (manager *Manager) ListSquadsByID(lastIndex int64, squadId string, networkType SquadNetworkType) (squads []*Squad, err error) {
	switch networkType {
	case MESH:
		squads, err = manager.SquadDBManager.GetSquadsByID(context.Background(), squadId, 100, lastIndex)
	case HOSTED:
		squads, err = manager.HostedSquadDBManager.GetHostedSquadsByID(context.Background(), squadId, 100, lastIndex)
	}
	return
}

func (manager *Manager) ListAllPeers(lastIndex int64) (peers []*Peer, err error) {

	peers, err = manager.PeerDBManager.GetPeers(context.Background(), 100, lastIndex)
	return
}

func (manager *Manager) ListPeersByID(lastIndex int64, id string) (peers []*Peer, err error) {

	peers, err = manager.PeerDBManager.GetPeersByID(context.Background(), id, 100, lastIndex)
	return
}

func (manager *Manager) ListPeersByName(lastIndex int64, name string) (peers []*Peer, err error) {

	peers, err = manager.PeerDBManager.GetPeersByName(context.Background(), name, 100, lastIndex)
	return
}

func (manager *Manager) UpdateSquadName(squadId string, squadName string) (err error) {
	err = manager.SquadDBManager.UpdateSquadName(context.Background(), squadId, squadName)
	return
}

func (manager *Manager) UpdateSquadAuthorizedMembers(squadId string, authorizedMembers string) (err error) {
	var squad *Squad
	if squad, err = manager.SquadDBManager.GetSquad(context.Background(), squadId); err != nil {
		return
	}
	for _, v := range squad.AuthorizedMembers {
		if v == authorizedMembers {
			err = fmt.Errorf("user already authorized")
			return
		}
	}
	err = manager.SquadDBManager.UpdateSquadAuthorizedMembers(context.Background(), squadId, append(squad.AuthorizedMembers, authorizedMembers))
	return
}

func (manager *Manager) UpdateSquadPassword(squadId string, password string) (err error) {
	pass, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return
	}
	err = manager.SquadDBManager.UpdateSquadName(context.Background(), squadId, string(pass))
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
