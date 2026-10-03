package socket

import (
	"net/http/httptest"
	"testing"
)

func TestWebSocketOriginPolicy(t *testing.T) {
	tests := []struct {
		name   string
		origin string
		want   bool
	}{
		{name: "native client without origin", want: true},
		{name: "same https origin", origin: "https://msg.trip-vn.com", want: true},
		{name: "same http origin", origin: "http://msg.trip-vn.com", want: true},
		{name: "cross site browser", origin: "https://example.com", want: false},
		{name: "invalid origin", origin: "://bad", want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest("GET", "https://msg.trip-vn.com/ws", nil)
			if test.origin != "" {
				request.Header.Set("Origin", test.origin)
			}
			if got := upgrader.CheckOrigin(request); got != test.want {
				t.Fatalf("CheckOrigin() = %v, want %v", got, test.want)
			}
		})
	}
}
