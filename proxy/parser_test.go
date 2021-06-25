package proxy

import (
	"fmt"
	"testing"
)

func TestParser(t *testing.T) {
	_, _, s := parse("", "SELECT count(*) FROM system.local")
	fmt.Println(s)
}
