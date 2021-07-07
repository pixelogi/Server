package manager

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type DbManager struct {
	*mongo.Client
	Db *mongo.Database
}

func NewDbManager(ctx context.Context, dbName string, host string, port int) (<-chan *DbManager, <-chan error) {
	dbManagerChan, errChan := make(chan *DbManager), make(chan error)
	go func() {
		client, err := mongo.NewClient(options.Client().ApplyURI(fmt.Sprintf("mongodb://%s:%d", host, port)))
		if err != nil {
			errChan <- err
			return
		}
		c, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		if err = client.Connect(c); err != nil {
			errChan <- err
			return
		}
		db := client.Database(dbName)
		dbManagerChan <- &DbManager{
			Client: client,
			Db:     db,
		}
		close(errChan)
	}()
	go func() {
		select {
		case <-ctx.Done():
			errChan <- ctx.Err()
		case <-errChan:
		}
	}()
	return dbManagerChan, errChan
}

func (dbm *DbManager) Init(collections ...string) {
	for _, name := range collections {
		if err := dbm.Db.CreateCollection(context.Background(), name); err != nil {
			fmt.Println(err)
		}
	}
}
