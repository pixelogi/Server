package manager

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"
)

type SquadDBManager struct {
	*mongo.Collection
}

const SQUAD_COLLECTION_NAME = "squads"

func NewSquadDBManager(host string,port int) (<-chan *SquadDBManager,<-chan error) {
	squadDBManagerCh,errCh := make(chan *SquadDBManager),make(chan error)
	go func() {
		dbManagerCh,errC := NewDbManager(context.Background(),DB_NAME,host,port)
		select {
		case dbManager := <-dbManagerCh:
			squadDBManagerCh <- &SquadDBManager{dbManager.Db.Collection(SQUAD_COLLECTION_NAME)}
		case err := <-errC:
			errCh <- err
		}
	}()
	return squadDBManagerCh,errCh
}