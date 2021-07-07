package manager

import (
	"crypto/rsa"
	"sync"

	"golang.org/x/crypto/bcrypt"
)

type SquadNetworkType = string

const (
	MESH   SquadNetworkType = "mesh"
	HOSTED SquadNetworkType = "hosted"
)

type Squad struct {
	Owner             string
	Name              string
	ID                string
	HostId            string
	NetworkType       SquadNetworkType
	SquadType         SquadType
	Password          string
	Members           []string
	AuthorizedMembers []*rsa.PublicKey
	*sync.RWMutex
}

func (squad *Squad) GetMembersLen() int {
	squad.RLock()
	defer squad.RUnlock()
	return len(squad.Members)
}

func (squad *Squad) Join(userId string) {
	squad.Lock()
	defer squad.Unlock()
	squad.Members = append(squad.Members, userId)
}

func (squad *Squad) Authenticate(password string) bool {
	squad.RLock()
	defer squad.RUnlock()
	err := bcrypt.CompareHashAndPassword([]byte(squad.Password), []byte(password))
	return err == nil
}
