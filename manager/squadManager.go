package manager

import (
	"sync"

	"golang.org/x/crypto/bcrypt"
)

type SquadNetworkType = string

const (
	MESH   SquadNetworkType = "mesh"
	HOSTED SquadNetworkType = "hosted"
)

type Squad struct {
	Owner       string
	Name        string
	ID          string
	HostId      string
	NetworkType SquadNetworkType
	SquadType   SquadType
	Password    string
	Members     []string
	Status      bool
	AuthType
	AuthorizedMembers []string
	mutex             *sync.RWMutex
}

func (squad *Squad) GetMembersLen() int {
	squad.mutex.RLock()
	defer squad.mutex.RUnlock()
	return len(squad.Members)
}

func (squad *Squad) Join(userId string) {
	squad.mutex.Lock()
	defer squad.mutex.Unlock()
	squad.Members = append(squad.Members, userId)
}

func (squad *Squad) Authenticate(password string) bool {
	squad.mutex.RLock()
	defer squad.mutex.RUnlock()
	err := bcrypt.CompareHashAndPassword([]byte(squad.Password), []byte(password))
	return err == nil
}
