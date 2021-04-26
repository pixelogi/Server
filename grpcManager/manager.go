package grpcManager

import (
	"fmt"
	"log"
	"sync"
)

type (
	GRPCManagerState uint8
	GRPCPeerState    uint8

	GRPCPeer struct {
		Conn  GrpcManager_LinkServer
		State GRPCPeerState
	}

	GRPCManager struct {
		State     GRPCManagerState
		GRPCPeers map[string]*GRPCPeer
		*sync.RWMutex
	}
)

const (
	ON GRPCManagerState = iota
	OFF
)

const (
	CONNECTED GRPCPeerState = iota
	SLEEP
)

func (manager *GRPCManager) AddPeer(peer GrpcManager_LinkServer, id string, req *Request) (err error) {
	fmt.Printf("adding peer %s\n", req.From)
	manager.Lock()
	manager.GRPCPeers[req.From] = &GRPCPeer{Conn: peer}
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
	delete(manager.GRPCPeers,req.From)
	return
}

func (manager *GRPCManager) manage(peer GrpcManager_LinkServer) (err error) {
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
