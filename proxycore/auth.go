package proxycore

import (
	"bytes"
	"fmt"
)

type defaultAuth struct {
	authId   string
	username string
	password string
}

func (d *defaultAuth) InitialResponse(authenticator string) ([]byte, error) {
	switch authenticator {
	case "com.datastax.bdp.cassandra.auth.DseAuthenticator":
		return []byte("PLAIN"), nil
	case "org.apache.cassandra.auth.PasswordAuthenticator":
		return d.makeToken(), nil
	}
	return nil, fmt.Errorf("unknown authenticator: %v", authenticator)
}

func (d *defaultAuth) EvaluateChallenge(token []byte) ([]byte, error) {
	if token == nil || bytes.Compare(token, []byte("PLAIN-START")) != 0 {
		return nil, fmt.Errorf("incorrect SASL challenge from server, expecting PLAIN-START, got: %v", string(token))
	}
	return d.makeToken(), nil
}

func (d *defaultAuth) makeToken() []byte {
	token := bytes.NewBuffer(make([]byte, 0, len(d.authId)+len(d.username)+len(d.password)+2))
	token.WriteString(d.authId)
	token.WriteByte(0)
	token.WriteString(d.username)
	token.WriteByte(0)
	token.WriteString(d.password)
	return token.Bytes()
}

func (d *defaultAuth) Success(token []byte) error {
	return nil
}

func NewDefaultAuth(username string, password string) Authenticator {
	return &defaultAuth{
		authId:   "",
		username: username,
		password: password,
	}
}
