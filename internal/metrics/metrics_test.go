package metrics

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandlerExposesCollectors(t *testing.T) {
	Commands.WithLabelValues("mirror").Inc()
	TelegramOps.WithLabelValues("send", ResultOK).Inc()
	RegisterGauge("mirrorbot_test_gauge", "test", func() float64 { return 3 })

	srv := httptest.NewServer(Handler())
	defer srv.Close()

	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatalf("scrape: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	out := string(body)

	for _, want := range []string{
		"mirrorbot_commands_total",
		"mirrorbot_telegram_ops_total",
		"mirrorbot_test_gauge 3",
		"go_goroutines", // default Go collector is present
	} {
		if !strings.Contains(out, want) {
			t.Errorf("metrics output missing %q", want)
		}
	}
}
