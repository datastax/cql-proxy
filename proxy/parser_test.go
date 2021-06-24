package proxy

import (
	"fmt"
	"testing"
)

func TestParser(t *testing.T) {
	s := parse("", "SELECT count(*) FROM system.local")
	fmt.Println(s)
}
