//go:build unit

package service

import (
	"context"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSharedAccountRetryAfterIsNotShortened(t *testing.T) {
	delay := openAIOAuth429SameAccountRetryDelay(http.Header{"Retry-After": {"90"}}, time.Now().Add(time.Second))
	require.GreaterOrEqual(t, delay, 89*time.Second)
}

func TestSharedAccountDeviceSessionsCannotShareWSConnection(t *testing.T) {
	account := &Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Extra: map[string]any{
		codexFingerprintModeExtraKey: "device", codexFingerprintSeedExtraKey: "11111111-1111-4111-8111-111111111111",
	}}
	first := http.Header{"X-Codex-Installation-Id": {"device"}, "Session-Id": {"alice"}}
	second := first.Clone()
	second.Set("session-id", "bob")
	require.NotEqual(t, normalizeOpenAIWSHandshakeCompatibility(account, first), normalizeOpenAIWSHandshakeCompatibility(account, second))
}

func TestSharedAccountProbeUsesBusinessConcurrency(t *testing.T) {
	cache := &stubConcurrencyCacheForTest{acquireResult: false}
	s := &AccountTestService{concurrencyService: NewConcurrencyService(cache)}
	account := &Account{ID: 72, Concurrency: 2}
	release, err := s.acquireTestAccountSlot(context.Background(), account)
	require.ErrorIs(t, err, ErrTestAccountBusy)
	require.Nil(t, release)
	cache.acquireResult = true
	release, err = s.acquireTestAccountSlot(context.Background(), account)
	require.NoError(t, err)
	release()
	require.Equal(t, []int64{72}, cache.releasedAccountIDs)
}

func TestSharedAccountProbeHonorsLatestCooldown(t *testing.T) {
	now := time.Now()
	short, long := now.Add(time.Minute), now.Add(10*time.Minute)
	account := &Account{ID: 72, RateLimitResetAt: &short, OverloadUntil: &long}
	var wait *TestAdmissionWaitError
	require.ErrorAs(t, accountTestCooldown(context.Background(), account, "gpt-5.5", now), &wait)
	require.Equal(t, long, wait.Until)
	require.NoError(t, accountTestCooldown(context.Background(), account, "gpt-5.5", long.Add(time.Second)))
}

func TestSharedAccountWSMetadataAndHandshakeShareIdentity(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/responses", nil)
	c.Request.Header.Set("session-id", "alice-session")
	account := &Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Extra: map[string]any{
		codexFingerprintModeExtraKey: "session", codexFingerprintSeedExtraKey: "11111111-1111-4111-8111-111111111111",
	}}
	body := []byte(`{"type":"response.create","client_metadata":{"installation_id":"original","session-id":"original","turn-id":"original"}}`)
	first, _, err := applyCodexWSIdentityRaw(c, account, body)
	require.NoError(t, err)
	headers := http.Header{}
	applyStagedCodexFingerprintHeaders(c, account, headers)
	require.Equal(t, headers.Get("session-id"), gjson.GetBytes(first, "client_metadata.session_id").String())
	require.Equal(t, headers.Get("x-codex-installation-id"), gjson.GetBytes(first, "client_metadata.installation_id").String())
	require.Equal(t, gjson.GetBytes(first, "client_metadata.session_id").String(), gjson.GetBytes(first, "client_metadata.session-id").String())
	second, _, err := applyCodexWSIdentityRaw(c, account, body)
	require.NoError(t, err)
	require.Equal(t, gjson.GetBytes(first, "client_metadata.thread_id").String(), gjson.GetBytes(second, "client_metadata.thread_id").String())
	require.NotEqual(t, gjson.GetBytes(first, "client_metadata.turn_id").String(), gjson.GetBytes(second, "client_metadata.turn_id").String())
	account.Extra[codexFingerprintModeExtraKey] = "off"
	_, _, err = applyCodexWSIdentityRaw(c, account, body)
	require.NoError(t, err)
	require.Nil(t, stagedCodexFingerprintIDs(c, account), "switching modes must clear previously staged IDs")
}

func TestSharedAccountExplicitRetryAfterParksAccountInsteadOfRapidRetry(t *testing.T) {
	svc := &OpenAIGatewayService{}
	account := &Account{ID: 2, Platform: PlatformOpenAI, Type: AccountTypeOAuth}
	headers := http.Header{"Retry-After": {"90"}}
	require.False(t, svc.ShouldRetryOpenAIOAuth429(account, headers, []byte(`{"error":{"type":"rate_limit_error"}}`)))
	svc.markOpenAIOAuth429RateLimited(context.Background(), account, headers, nil)
	block := svc.peekOpenAIAccountRuntimeBlock(account)
	require.True(t, block.blocked)
	require.Greater(t, time.Until(block.until), 89*time.Second)
}

func TestSharedAccountProbeMapsCooldownModelOnlyOnce(t *testing.T) {
	reset := time.Now().Add(time.Minute).UTC().Format(time.RFC3339)
	account := &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth, Credentials: map[string]any{
		"model_mapping": map[string]any{"alias": "target", "target": "different"},
	}, Extra: map[string]any{"model_rate_limits": map[string]any{"target": map[string]any{"rate_limit_reset_at": reset}}}}
	require.Error(t, accountTestCooldown(context.Background(), account, accountTestAdmissionModel(account, "alias"), time.Now()))
}
