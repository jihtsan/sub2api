//go:build unit

package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestAccountTrafficHTTPFailureKeepsLocalStatusAndProtocol(t *testing.T) {
	for _, anthropic := range []bool{false, true} {
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
		local := (&service.AccountTrafficLimitError{Reason: "local budget", RetryAfter: 2 * time.Second}).FailoverError()
		h := &OpenAIGatewayHandler{}
		if anthropic {
			h.handleAnthropicFailoverExhausted(c, local, false)
		} else {
			h.handleFailoverExhausted(c, local, false)
		}
		require.Equal(t, http.StatusTooManyRequests, recorder.Code)
		require.Equal(t, "2", recorder.Header().Get("Retry-After"))
		require.Equal(t, "local budget", gjson.Get(recorder.Body.String(), "error.message").String())
		if anthropic {
			require.Equal(t, "error", gjson.Get(recorder.Body.String(), "type").String())
		}
	}
}
