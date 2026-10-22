package github

import (
	"errors"
	"fmt"
	"testing"
)

func TestExplainSetup(t *testing.T) {
	missing := fmt.Errorf("authentication token not found for host github.com")
	if got := explainSetup(missing); !errors.Is(got, errNotLoggedIn) {
		t.Errorf("missing token = %v, want errNotLoggedIn", got)
	}
	other := errors.New("bad host")
	if got := explainSetup(other); got != other {
		t.Errorf("other errors should pass through, got %v", got)
	}
}
