package manager

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"
)

type PeerDBManager struct {
	*mongo.Collection
}

const PEER_COLLECTION_NAME = "peers"

func NewPeerDBManager(host string,port int) (<-chan *PeerDBManager,<-chan error) {
	peerDBManagerCh,errCh := make(chan *PeerDBManager),make(chan error)
	go func() {
		dbManagerCh,errC := NewDbManager(context.Background(),DB_NAME,host,port)
		select {
		case dbManager := <-dbManagerCh:
			peerDBManagerCh <- &PeerDBManager{dbManager.Db.Collection(PEER_COLLECTION_NAME)}
		case err := <-errC:
			errCh <- err
		}
	}()
	return peerDBManagerCh,errCh
}