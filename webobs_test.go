package webobs

import (
	_ "errors"
	"fmt"
	_ "log"
	"testing"
	_ "time"
)

func doTestClients() error {
	s := newServer()
	c := s.addNewclientobs("test", nil)
	fmt.Println("clients", s.clients)
	s.removeclientobs("test", c.id)
	fmt.Println("clients", s.clients)
	return nil
}

func TestCache(t *testing.T) {

	err := doTestClients()
	switch {
	case err != nil:
		t.Errorf("cache elements test failed", err)
	}
}
