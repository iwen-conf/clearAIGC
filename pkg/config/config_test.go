package config

import (
	"testing"
	"time"

	"github.com/spf13/viper"
)

func TestSetDefaultsDisablesHTTPWriteTimeoutForSSE(t *testing.T) {
	v := viper.New()

	setDefaults(v)

	if got := v.GetDuration("http.write_timeout"); got != 0*time.Second {
		t.Fatalf("expected http.write_timeout default to be disabled for SSE, got %s", got)
	}
}
