package grpcManager

import (
	"context"
	"log"
	"sync"
)

const (
	INIT = "init"
	TEST = "test"
)

type GRPCManagerService struct {
	*UnimplementedGrpcManagerServer
	Manager *GRPCManager
}

func NewGRPCManagerService() (service *GRPCManagerService) {
	manager := &GRPCManager{
		State:     ON,
		GRPCPeers: make(map[string]*GRPCPeer),
		RWMutex:   &sync.RWMutex{},
	}
	service = &GRPCManagerService{
		Manager: manager,
	}
	return
}

func (service *GRPCManagerService) Link(stream GrpcManager_LinkServer) (err error) {
	done, errch := make(chan struct{}), make(chan error)
	go func() {
		req, err := stream.Recv()
		if err != nil {
			errch <- err
			return
		} else if service.Manager.State == ON {
			if err := service.Manager.AddPeer(stream, req.From, req); err != nil {
				errch <- err
				return
			}
			done <- struct{}{}
		}
	}()
	select {
	case <-stream.Context().Done():
		err = stream.Context().Err()
		log.Println(err)
		return
	case <-done:
		log.Println("Link with peer is successful")
		return
	case err = <-errch:
		log.Println(err)
		return
	}
}

func (service *GRPCManagerService) ListPeers(ctx context.Context, peerListRequest *PeerListRequest) (peerListResponse *PeerListResponse, err error) {
	list, errch := make(chan []*Peer), make(chan error)
	go func() {
		peerList := []*Peer{}
		count := 0
		for id := range service.Manager.GRPCPeers {
			if count < int(peerListRequest.Number) {
				peer := &Peer{
					Id:   id,
					Name: "unknown",
				}
				peerList = append(peerList, peer)
			} else {
				break
			}
		}
		list <- peerList
	}()
	select {
	case <-ctx.Done():
		log.Println(err)
		err = ctx.Err()
		return
	case peerList := <-list:
		peerListResponse = &PeerListResponse{
			Peers:     peerList,
			Success:   true,
			LastIndex: peerListRequest.LastIndex + int32(len(peerList)),
		}
		return
	case err = <-errch:
		log.Println(err)
		return
	}
}
