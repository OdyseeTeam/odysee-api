package sdkrouter

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/OdyseeTeam/odysee-api/internal/storage"
	"github.com/OdyseeTeam/odysee-api/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/volatiletech/null"
	"github.com/volatiletech/sqlboiler/boil"
	"github.com/ybbus/jsonrpc/v2"
)

const (
	sdkTestLeaseOwnerA = "owner-a"
	sdkTestLeaseOwnerB = "owner-b"
	sdkTestSentinelURL = "what"
)

type sdkTimeoutError struct{}

func (sdkTimeoutError) Error() string {
	return "deadline exceeded"
}

func (sdkTimeoutError) Timeout() bool {
	return true
}

func (sdkTimeoutError) Temporary() bool {
	return true
}

func TestClassifySDKHealth(t *testing.T) {
	assert.Equal(t, sdkProbeHealthy, classifySDKHealth(&jsonrpc.RPCResponse{}, nil))
	assert.Equal(t, sdkProbeDeadLoop, classifySDKHealth(&jsonrpc.RPCResponse{
		Error: &jsonrpc.RPCError{Message: "connection is not available"},
	}, nil))
	assert.Equal(t, sdkProbeStarting, classifySDKHealth(&jsonrpc.RPCResponse{
		Error: &jsonrpc.RPCError{Message: "daemon is still starting"},
	}, nil))
	assert.Equal(t, sdkProbeStarting, classifySDKHealth(&jsonrpc.RPCResponse{
		Error: &jsonrpc.RPCError{Message: "wallet is not loaded"},
	}, nil))
	assert.Equal(t, sdkProbeStarting, classifySDKHealth(&jsonrpc.RPCResponse{
		Error: &jsonrpc.RPCError{Message: "components have not yet started"},
	}, nil))
	assert.Equal(t, sdkProbeTimeout, classifySDKHealth(nil, sdkTimeoutError{}))
	assert.Equal(t, sdkProbeDeadLoop, classifySDKHealth(nil, errors.New("timeout awaiting response headers")))
	assert.Equal(t, sdkProbeError, classifySDKHealth(nil, errors.New("connection refused")))
}

func TestProbeSDKHealthHungServerIsDeadLoop(t *testing.T) {
	done := make(chan struct{})
	server := httptest.NewServer(hungSDKProbeHandler{done: done})

	cfg := sdkHealthRuntimeConfig{
		HealthProbeTimeout: 20 * time.Millisecond,
		HealthSentinelURL:  sdkTestSentinelURL,
	}

	result := probeSDKHealth(server.URL, cfg)
	close(done)
	server.Close()

	assert.Equal(t, sdkProbeDeadLoop, result)
}

type hungSDKProbeHandler struct {
	done <-chan struct{}
}

func (h hungSDKProbeHandler) ServeHTTP(_ http.ResponseWriter, _ *http.Request) {
	<-h.done
}

func TestSDKFleetCircuitRequiresMinimumCount(t *testing.T) {
	assert.False(t, sdkFleetCircuitOpen(2, 5, 0.3))
	assert.True(t, sdkFleetCircuitOpen(3, 5, 0.3))
	assert.False(t, sdkFleetCircuitOpen(5, 5, 1))
}

func TestSDKDeadLoopThresholdResetsOnNonActionableProbe(t *testing.T) {
	r := NewWithServers(&models.LbrynetServer{Name: "sdk", Address: "http://sdk"})

	assert.False(t, r.markSDKDeadLoop("http://sdk", 2))
	r.markSDKNonActionable("http://sdk")
	assert.False(t, r.markSDKDeadLoop("http://sdk", 2))
	assert.True(t, r.markSDKDeadLoop("http://sdk", 2))
}

func TestSDKLeaseAcquireCooldownAndRecoveryReset(t *testing.T) {
	_, err := models.SDKReconnectLeases().DeleteAllG()
	require.NoError(t, err)

	address := "http://lease-sdk"
	cfg := sdkHealthRuntimeConfig{
		HealthCheckInterval:    time.Second,
		HealthProbeTimeout:     time.Second,
		ReconnectCooldown:      time.Hour,
		ReconnectLeaseTTL:      time.Minute,
		ReconnectObservation:   time.Minute,
		HealthFailureThreshold: 1,
		ReconnectMaxConcurrent: 1,
		ReconnectMaxAttempts:   2,
		UnhealthyFleetFraction: 1,
		HealthSentinelURL:      sdkTestSentinelURL,
	}
	store := &sdkLeaseStore{db: storage.DB, owner: sdkTestLeaseOwnerA}
	otherStore := &sdkLeaseStore{db: storage.DB, owner: sdkTestLeaseOwnerB}

	lease, decision, err := store.acquire(address, cfg)
	require.NoError(t, err)
	require.NotNil(t, lease)
	assert.Empty(t, decision)

	contendedLease, decision, err := otherStore.acquire("http://other-sdk", cfg)
	require.NoError(t, err)
	assert.Nil(t, contendedLease)
	assert.Equal(t, sdkReconnectDecisionLeaseUnavailable, decision)

	err = store.release(activeRecoveryFromLease(lease, cfg), false)
	require.NoError(t, err)

	cooldownLease, decision, err := store.acquire(address, cfg)
	require.NoError(t, err)
	assert.Nil(t, cooldownLease)
	assert.Equal(t, sdkReconnectDecisionCooldown, decision)

	recoveredAddress := "http://recovered-sdk"
	recoveredLease, decision, err := store.acquire(recoveredAddress, cfg)
	require.NoError(t, err)
	require.NotNil(t, recoveredLease)
	assert.Empty(t, decision)

	err = store.release(activeRecoveryFromLease(recoveredLease, cfg), true)
	require.NoError(t, err)

	endpointLease, err := models.FindSDKReconnectLeaseG(sdkEndpointLeaseKey(recoveredAddress))
	require.NoError(t, err)
	assert.Equal(t, 0, endpointLease.Attempts)
	assert.False(t, endpointLease.Owner.Valid)
	assert.False(t, endpointLease.LeaseExpiresAt.Valid)
}

func TestSDKLeaseMaxAttemptsResetAfterHealthyProbe(t *testing.T) {
	_, err := models.SDKReconnectLeases().DeleteAllG()
	require.NoError(t, err)

	address := "http://max-attempt-sdk"
	cfg := sdkHealthRuntimeConfig{
		ReconnectLeaseTTL:      time.Minute,
		ReconnectObservation:   time.Minute,
		ReconnectMaxConcurrent: 1,
		ReconnectMaxAttempts:   2,
		UnhealthyFleetFraction: 1,
		HealthSentinelURL:      sdkTestSentinelURL,
	}
	store := &sdkLeaseStore{db: storage.DB, owner: sdkTestLeaseOwnerA}

	firstLease, decision, err := store.acquire(address, cfg)
	require.NoError(t, err)
	require.NotNil(t, firstLease)
	assert.Empty(t, decision)
	require.NoError(t, store.release(activeRecoveryFromLease(firstLease, cfg), false))

	secondLease, decision, err := store.acquire(address, cfg)
	require.NoError(t, err)
	require.NotNil(t, secondLease)
	assert.Empty(t, decision)
	require.NoError(t, store.release(activeRecoveryFromLease(secondLease, cfg), false))

	exhaustedLease, decision, err := store.acquire(address, cfg)
	require.NoError(t, err)
	assert.Nil(t, exhaustedLease)
	assert.Equal(t, sdkReconnectDecisionMaxAttemptsExhausted, decision)

	claimed, err := store.claimMaxAttemptsAlert(address)
	require.NoError(t, err)
	assert.True(t, claimed)
	claimed, err = store.claimMaxAttemptsAlert(address)
	require.NoError(t, err)
	assert.False(t, claimed)

	require.NoError(t, store.confirmHealthy(address, nil))
	endpointLease, err := models.FindSDKReconnectLeaseG(sdkEndpointLeaseKey(address))
	require.NoError(t, err)
	assert.False(t, endpointLease.MaxAttemptsAlertedAt.Valid)

	recoveredLease, decision, err := store.acquire(address, cfg)
	require.NoError(t, err)
	require.NotNil(t, recoveredLease)
	assert.Empty(t, decision)
}

func TestSDKLeaseConfirmHealthyDoesNotResetPeerOwnedLease(t *testing.T) {
	_, err := models.SDKReconnectLeases().DeleteAllG()
	require.NoError(t, err)

	address := "http://peer-owned-sdk"
	cfg := sdkHealthRuntimeConfig{
		ReconnectLeaseTTL:      time.Minute,
		ReconnectObservation:   time.Minute,
		ReconnectMaxConcurrent: 1,
		ReconnectMaxAttempts:   3,
		UnhealthyFleetFraction: 1,
		HealthSentinelURL:      sdkTestSentinelURL,
	}
	store := &sdkLeaseStore{db: storage.DB, owner: sdkTestLeaseOwnerA}
	otherStore := &sdkLeaseStore{db: storage.DB, owner: sdkTestLeaseOwnerB}

	lease, decision, err := store.acquire(address, cfg)
	require.NoError(t, err)
	require.NotNil(t, lease)
	assert.Empty(t, decision)

	endpointLease, err := models.FindSDKReconnectLeaseG(sdkEndpointLeaseKey(address))
	require.NoError(t, err)
	endpointLease.Attempts = 2
	endpointLease.MaxAttemptsAlertedAt = null.TimeFrom(time.Now())
	_, err = endpointLease.UpdateG(boil.Whitelist(
		models.SDKReconnectLeaseColumns.Attempts,
		models.SDKReconnectLeaseColumns.MaxAttemptsAlertedAt,
		models.SDKReconnectLeaseColumns.UpdatedAt,
	))
	require.NoError(t, err)

	require.NoError(t, otherStore.confirmHealthy(address, nil))

	endpointLease, err = models.FindSDKReconnectLeaseG(sdkEndpointLeaseKey(address))
	require.NoError(t, err)
	assert.Equal(t, 2, endpointLease.Attempts)
	assert.Equal(t, sdkTestLeaseOwnerA, endpointLease.Owner.String)
	assert.True(t, endpointLease.LeaseExpiresAt.Valid)
	assert.True(t, endpointLease.MaxAttemptsAlertedAt.Valid)

	require.NoError(t, store.confirmHealthy(address, activeRecoveryFromLease(lease, cfg)))

	endpointLease, err = models.FindSDKReconnectLeaseG(sdkEndpointLeaseKey(address))
	require.NoError(t, err)
	assert.Equal(t, 0, endpointLease.Attempts)
	assert.False(t, endpointLease.Owner.Valid)
	assert.False(t, endpointLease.LeaseExpiresAt.Valid)
	assert.False(t, endpointLease.MaxAttemptsAlertedAt.Valid)
}

func TestSDKLeaseExpiredGlobalSlotCanBeTakenOver(t *testing.T) {
	_, err := models.SDKReconnectLeases().DeleteAllG()
	require.NoError(t, err)

	cfg := sdkHealthRuntimeConfig{
		ReconnectLeaseTTL:      time.Hour,
		ReconnectObservation:   time.Minute,
		ReconnectMaxConcurrent: 1,
		ReconnectMaxAttempts:   3,
		UnhealthyFleetFraction: 1,
		HealthSentinelURL:      sdkTestSentinelURL,
	}
	store := &sdkLeaseStore{db: storage.DB, owner: sdkTestLeaseOwnerA}
	otherStore := &sdkLeaseStore{db: storage.DB, owner: sdkTestLeaseOwnerB}

	lease, decision, err := store.acquire("http://slot-owner-sdk", cfg)
	require.NoError(t, err)
	require.NotNil(t, lease)
	assert.Empty(t, decision)

	contendedLease, decision, err := otherStore.acquire("http://slot-contender-sdk", cfg)
	require.NoError(t, err)
	assert.Nil(t, contendedLease)
	assert.Equal(t, sdkReconnectDecisionLeaseUnavailable, decision)

	slotLease, err := models.FindSDKReconnectLeaseG(lease.slotKey)
	require.NoError(t, err)
	slotLease.LeaseExpiresAt = null.TimeFrom(time.Now().Add(-time.Hour))
	_, err = slotLease.UpdateG(boil.Whitelist(
		models.SDKReconnectLeaseColumns.LeaseExpiresAt,
		models.SDKReconnectLeaseColumns.UpdatedAt,
	))
	require.NoError(t, err)

	takeoverLease, decision, err := otherStore.acquire("http://slot-contender-sdk", cfg)
	require.NoError(t, err)
	require.NotNil(t, takeoverLease)
	assert.Empty(t, decision)
	assert.Equal(t, lease.slotKey, takeoverLease.slotKey)
}

func TestSDKLeaseRejectsInvalidGlobalSlotKeys(t *testing.T) {
	store := &sdkLeaseStore{db: storage.DB, owner: sdkTestLeaseOwnerA}
	cfg := sdkHealthRuntimeConfig{ReconnectMaxConcurrent: 1}

	err := store.release(&sdkActiveRecovery{
		endpointKey: sdkEndpointLeaseKey("http://sdk"),
		slotKey:     "bad-slot",
	}, false)
	assert.Error(t, err)

	err = store.renew(&sdkActiveRecovery{
		endpointKey: sdkEndpointLeaseKey("http://sdk"),
		slotKey:     sdkGlobalLeaseKey(1),
	}, cfg)
	assert.Error(t, err)
}
