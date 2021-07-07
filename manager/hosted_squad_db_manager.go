package manager

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"
)

type HostedSquadDBManager struct {
	*mongo.Collection
}

const HOSTED_SQUAD_COLLECTION_NAME = "hosted_squads"

func NewHostedSquadDBManager(host string,port int) (<-chan *HostedSquadDBManager,<-chan error) {
	hostedSquadDBManagerCh,errCh := make(chan *HostedSquadDBManager),make(chan error)
	go func() {
		dbManagerCh,errC := NewDbManager(context.Background(),DB_NAME,host,port)
		select {
		case dbManager := <-dbManagerCh:
			hostedSquadDBManagerCh <- &HostedSquadDBManager{dbManager.Db.Collection(SQUAD_COLLECTION_NAME)}
		case err := <-errC:
			errCh <- err
		}
	}()
	return hostedSquadDBManagerCh,errCh
}