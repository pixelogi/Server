package manager

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"

	"github.com/google/uuid"
)

type AuthType string

type AuthManager struct {
	AuthTokenPending map[string]string
	AuthTokenValid   map[string]string
}

const (
	PWD AuthType = "PWD"
	RSA AuthType = "RSA"
)

func NewAuthManager() (authManager *AuthManager) {
	authManager = &AuthManager{
		AuthTokenPending: make(map[string]string),
		AuthTokenValid:   make(map[string]string),
	}
	return
}

func (am *AuthManager) GenerateAuthToken(peerId string, publicKey string) (encryptedToken []byte, err error) {
	encryptedTokenCh, errCh := make(chan []byte), make(chan error)
	go func() {
		token, e := uuid.NewRandom()
		if e != nil {
			errCh <- e
			return
		}
		fmt.Println("token:", token)
		pubKey, e := am.parsePublicKey(publicKey)
		if e != nil {
			errCh <- fmt.Errorf("error in parse pub key : %v", e)
			return
		}
		encryptedMsg, e := am.encryptWithPublicKey([]byte(token.String()), pubKey)
		if e != nil {
			errCh <- fmt.Errorf("error in encrypt with key : %v", e)
			return
		}
		am.AuthTokenPending[peerId] = token.String()
		encryptedTokenCh <- encryptedMsg
	}()
	select {
	case encryptedToken = <-encryptedTokenCh:
	case err = <-errCh:
	}
	return
}

func (am *AuthManager) parsePublicKey(publicKey string) (pubKey *rsa.PublicKey, err error) {
	pub := []byte(publicKey)
	block, _ := pem.Decode(pub)
	b := block.Bytes
	pubKey, err = x509.ParsePKCS1PublicKey(b)
	if err != nil {
		return
	}
	return
}

func (am *AuthManager) parsePrivKey(privateKey string) (pubKey *rsa.PrivateKey, err error) {
	pub := []byte(privateKey)
	block, _ := pem.Decode(pub)
	b := block.Bytes
	pubKey, err = x509.ParsePKCS1PrivateKey(b)
	if err != nil {
		return
	}
	return
}

func (am *AuthManager) encryptWithPublicKey(msg []byte, publicKey *rsa.PublicKey) (encryptedMsg []byte, err error) {
	encryptedMsg, err = rsa.EncryptPKCS1v15(rand.Reader, publicKey, msg)
	return
}
