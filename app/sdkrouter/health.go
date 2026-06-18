package sdkrouter

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/OdyseeTeam/odysee-api/apps/lbrytv/config"
	"github.com/OdyseeTeam/odysee-api/internal/metrics"
	"github.com/OdyseeTeam/odysee-api/internal/monitor"
	"github.com/OdyseeTeam/odysee-api/models"

	"github.com/ybbus/jsonrpc/v2"
)

const (
	sdkMethodResolve         = "resolve"
	sdkMethodWalletReconnect = "wallet_reconnect"
	sdkMonitorContextKey     = "sdk"
	sdkParamURLs             = "urls"
	sdkFleetCircuitMinCount  = 3

	sdkProbeHealthy  sdkProbeResult = "healthy"
	sdkProbeDeadLoop sdkProbeResult = "dead_loop"
	sdkProbeStarting sdkProbeResult = "starting"
	sdkProbeTimeout  sdkProbeResult = "timeout"
	sdkProbeError    sdkProbeResult = "error"

	sdkReconnectDecisionLeaseUnavailable     = "lease_unavailable"
	sdkReconnectDecisionCooldown             = "cooldown"
	sdkReconnectDecisionFleetCircuitOpen     = "fleet_circuit_open"
	sdkReconnectDecisionRPCRejected          = "rpc_rejected"
	sdkReconnectDecisionObservationTimeout   = "observation_timeout"
	sdkReconnectDecisionMaxAttemptsExhausted = "max_attempts_exhausted"
	sdkReconnectDecisionSlotReleased         = "slot_released"
	sdkReconnectDecisionReconnectDisabled    = "reconnect_disabled"
	sdkReconnectDecisionLeaseError           = "lease_error"
	sdkReconnectDecisionLeaseRenewFailed     = "lease_renew_failed"
)

var sdkHealthOwner = newSDKHealthOwner()

type sdkProbeResult string

type sdkHealthRuntimeConfig struct {
	ReconnectEnabled       bool
	HealthCheckInterval    time.Duration
	HealthProbeTimeout     time.Duration
	ReconnectCooldown      time.Duration
	ReconnectLeaseTTL      time.Duration
	ReconnectObservation   time.Duration
	HealthFailureThreshold int
	ReconnectMaxConcurrent int
	ReconnectMaxAttempts   int
	UnhealthyFleetFraction float64
	HealthSentinelURL      string
}

type sdkHealthState struct {
	recovery             *sdkActiveRecovery
	consecutiveDeadLoops int
}

type sdkActiveRecovery struct {
	endpointKey string
	slotKey     string
	deadline    time.Time
	attempts    int
}

type sdkHealthProbe struct {
	server *models.LbrynetServer
	result sdkProbeResult
}

type sdkFleetCircuitCount struct {
	total     int
	unhealthy int
}

func (r *Router) WatchHealth() {
	if !config.GetSDKHealthCheckEnabled() {
		return
	}

	cfg, err := loadSDKHealthRuntimeConfig()
	if err != nil {
		logger.Log().Errorf("SDK health watcher config error: %s", err)
		monitor.ErrorToSentry(err)
		return
	}

	var store *sdkLeaseStore
	if cfg.ReconnectEnabled {
		store, err = newSDKLeaseStore(sdkHealthOwner)
		if err != nil {
			logger.Log().Errorf("SDK reconnect lease store unavailable: %s", err)
			monitor.ErrorToSentry(err)
		}
	}

	logger.Log().Infof("SDK router watching health on %d instances", len(r.GetAll()))
	r.sleepBeforeHealthWatch(cfg)
	r.runHealthCheckRecovering(cfg, store)

	ticker := time.NewTicker(cfg.HealthCheckInterval)
	defer ticker.Stop()
	for {
		<-ticker.C
		r.runHealthCheckRecovering(cfg, store)
	}
}

func (r *Router) sleepBeforeHealthWatch(cfg sdkHealthRuntimeConfig) {
	maxJitter := int64(cfg.HealthCheckInterval / time.Second)
	if maxJitter <= 1 {
		return
	}
	jitter, err := randomHealthWatchJitter(maxJitter)
	if err != nil {
		jitter = time.Now().UnixNano() % maxJitter
	}
	time.Sleep(time.Duration(jitter) * time.Second)
}

func randomHealthWatchJitter(maxJitter int64) (int64, error) {
	jitter, err := rand.Int(rand.Reader, big.NewInt(maxJitter))
	if err != nil {
		return 0, err
	}
	return jitter.Int64(), nil
}

func loadSDKHealthRuntimeConfig() (sdkHealthRuntimeConfig, error) {
	err := config.ValidateSDKHealthConfig()
	if err != nil {
		return sdkHealthRuntimeConfig{}, err
	}

	return sdkHealthRuntimeConfig{
		ReconnectEnabled:       config.GetSDKReconnectEnabled(),
		HealthCheckInterval:    config.GetSDKHealthCheckInterval(),
		HealthProbeTimeout:     config.GetSDKHealthProbeTimeout(),
		ReconnectCooldown:      config.GetSDKReconnectCooldown(),
		ReconnectLeaseTTL:      config.GetSDKReconnectLeaseTTL(),
		ReconnectObservation:   config.GetSDKReconnectObservationWindow(),
		HealthFailureThreshold: config.GetSDKHealthFailureThreshold(),
		ReconnectMaxConcurrent: config.GetSDKReconnectMaxConcurrent(),
		ReconnectMaxAttempts:   config.GetSDKReconnectMaxAttempts(),
		UnhealthyFleetFraction: config.GetSDKUnhealthyFleetFraction(),
		HealthSentinelURL:      config.GetSDKHealthSentinelURL(),
	}, nil
}

func (r *Router) runHealthCheck(cfg sdkHealthRuntimeConfig, store *sdkLeaseStore) {
	servers := r.GetAll()
	if len(servers) == 0 {
		return
	}

	probes := make([]sdkHealthProbe, 0, len(servers))
	for _, server := range servers {
		result := probeSDKHealth(server.Address, cfg)
		probes = append(probes, sdkHealthProbe{server: server, result: result})
		metrics.LbrynetHealthProbeTotal.WithLabelValues(server.Address, string(result)).Inc()
		if result == sdkProbeHealthy {
			metrics.LbrynetInstanceHealthy.WithLabelValues(server.Address).Set(1)
		} else {
			metrics.LbrynetInstanceHealthy.WithLabelValues(server.Address).Set(0)
		}
	}

	fleetCircuits := sdkFleetCircuitOpenByGroup(probes, cfg.UnhealthyFleetFraction)
	for _, probe := range probes {
		group := sdkServerGroup(probe.server.Address)
		fleetCircuitOpen := fleetCircuits[group]
		r.handleHealthProbe(cfg, store, probe.server, probe.result, fleetCircuitOpen)
	}
}

func probeSDKHealth(address string, cfg sdkHealthRuntimeConfig) sdkProbeResult {
	client := jsonrpc.NewClientWithOpts(address, &jsonrpc.RPCClientOpts{
		HTTPClient: &http.Client{Timeout: cfg.HealthProbeTimeout},
	})
	resp, err := client.Call(sdkMethodResolve, map[string]interface{}{sdkParamURLs: cfg.HealthSentinelURL})
	result := classifySDKHealth(resp, err)
	if result == sdkProbeError {
		logger.Log().Warnf("SDK health probe error for %s: %s", address, sdkHealthErrorMessage(resp, err))
	}
	return result
}

func (r *Router) handleHealthProbe(cfg sdkHealthRuntimeConfig, store *sdkLeaseStore, server *models.LbrynetServer, result sdkProbeResult, fleetCircuitOpen bool) {
	address := server.Address
	if result == sdkProbeHealthy {
		recovery := r.markSDKHealthy(address)
		if store != nil {
			err := store.confirmHealthy(address, recovery)
			if err != nil {
				logger.Log().Errorf("failed to reset SDK reconnect state for %s after healthy probe: %s", address, err)
				decisionMetric(address, sdkReconnectDecisionLeaseError)
				return
			}
		}
		if recovery != nil {
			metrics.LbrynetWalletReconnectRecoveredTotal.WithLabelValues(address).Inc()
			decisionMetric(address, sdkReconnectDecisionSlotReleased)
		}
		return
	}

	recovery := r.currentSDKRecovery(address)
	if recovery != nil {
		r.handleActiveSDKRecovery(cfg, store, address, recovery)
		return
	}

	if result != sdkProbeDeadLoop {
		r.markSDKNonActionable(address)
		return
	}

	if !r.markSDKDeadLoop(address, cfg.HealthFailureThreshold) {
		return
	}
	if fleetCircuitOpen {
		decisionMetric(address, sdkReconnectDecisionFleetCircuitOpen)
		return
	}
	if !cfg.ReconnectEnabled {
		decisionMetric(address, sdkReconnectDecisionReconnectDisabled)
		return
	}
	if store == nil {
		decisionMetric(address, sdkReconnectDecisionLeaseError)
		return
	}

	lease, decision, err := store.acquire(address, cfg)
	if err != nil {
		logger.Log().Errorf("failed to acquire SDK reconnect lease for %s: %s", address, err)
		decisionMetric(address, sdkReconnectDecisionLeaseError)
		return
	}
	if lease == nil {
		if decision == sdkReconnectDecisionMaxAttemptsExhausted {
			r.reportSDKMaxAttemptsExhaustedOnce(store, address, "max attempts exhausted")
		} else {
			decisionMetric(address, decision)
		}
		return
	}

	metrics.LbrynetWalletReconnectAttemptTotal.WithLabelValues(address).Inc()
	logger.Log().Warnf("attempting SDK wallet_reconnect for %s", address)
	err = callSDKWalletReconnect(address, cfg)
	if err != nil {
		rpcRecovery := activeRecoveryFromLease(lease, cfg)
		releaseErr := store.release(rpcRecovery, false)
		if releaseErr != nil {
			logger.Log().Errorf("failed to release rejected SDK reconnect lease for %s: %s", address, releaseErr)
		}
		logger.Log().Warnf("SDK wallet_reconnect rejected for %s: %s", address, err)
		decisionMetric(address, sdkReconnectDecisionRPCRejected)
		if lease.attempts >= cfg.ReconnectMaxAttempts {
			r.reportSDKMaxAttemptsExhaustedOnce(store, address, "wallet_reconnect rejected")
		}
		return
	}

	metrics.LbrynetWalletReconnectAcceptedTotal.WithLabelValues(address).Inc()
	logger.Log().Infof("SDK wallet_reconnect accepted for %s", address)
	r.setSDKRecovery(address, activeRecoveryFromLease(lease, cfg))
}

func (r *Router) handleActiveSDKRecovery(cfg sdkHealthRuntimeConfig, store *sdkLeaseStore, address string, recovery *sdkActiveRecovery) {
	if time.Now().After(recovery.deadline) {
		if r.clearSDKRecovery(address, recovery) && store != nil {
			err := store.release(recovery, false)
			if err != nil {
				logger.Log().Errorf("failed to release timed out SDK reconnect lease for %s: %s", address, err)
				decisionMetric(address, sdkReconnectDecisionLeaseError)
				return
			}
		}
		decisionMetric(address, sdkReconnectDecisionObservationTimeout)
		if recovery.attempts >= cfg.ReconnectMaxAttempts {
			r.reportSDKMaxAttemptsExhaustedOnce(store, address, "observation window expired")
		}
		return
	}
	if store == nil {
		return
	}
	err := store.renew(recovery, cfg)
	if err != nil {
		logger.Log().Warnf("failed to renew SDK reconnect lease for %s: %s", address, err)
		decisionMetric(address, sdkReconnectDecisionLeaseRenewFailed)
	}
}

func callSDKWalletReconnect(address string, cfg sdkHealthRuntimeConfig) error {
	client := jsonrpc.NewClientWithOpts(address, &jsonrpc.RPCClientOpts{
		HTTPClient: &http.Client{Timeout: cfg.HealthProbeTimeout},
	})
	resp, err := client.Call(sdkMethodWalletReconnect)
	if err != nil {
		return err
	}
	if resp != nil && resp.Error != nil {
		return resp.Error
	}
	return nil
}

func classifySDKHealth(resp *jsonrpc.RPCResponse, err error) sdkProbeResult {
	if err != nil {
		if sdkDeadLoopTransportTimeout(err.Error()) {
			return sdkProbeDeadLoop
		}
		if timeoutError(err) {
			return sdkProbeTimeout
		}
		return classifySDKHealthMessage(err.Error(), sdkProbeError)
	}
	if resp == nil {
		return sdkProbeError
	}
	if resp.Error == nil {
		return sdkProbeHealthy
	}
	return classifySDKHealthMessage(resp.Error.Message, sdkProbeError)
}

func sdkDeadLoopTransportTimeout(message string) bool {
	msg := strings.ToLower(message)
	if strings.Contains(msg, "timeout awaiting response headers") {
		return true
	}
	if strings.Contains(msg, "client.timeout exceeded") && strings.Contains(msg, "awaiting headers") {
		return true
	}
	if strings.Contains(msg, "deadline exceeded") && strings.Contains(msg, "awaiting headers") {
		return true
	}
	return false
}

func classifySDKHealthMessage(message string, fallback sdkProbeResult) sdkProbeResult {
	msg := strings.ToLower(message)
	if strings.Contains(msg, "connection is not available") {
		return sdkProbeDeadLoop
	}
	if strings.Contains(msg, "still starting") ||
		strings.Contains(msg, "starting") ||
		strings.Contains(msg, "wallet is not loaded") ||
		strings.Contains(msg, "components have not yet started") {
		return sdkProbeStarting
	}
	if strings.Contains(msg, "timeout") || strings.Contains(msg, "deadline exceeded") {
		return sdkProbeTimeout
	}
	return fallback
}

func sdkHealthErrorMessage(resp *jsonrpc.RPCResponse, err error) string {
	if err != nil {
		return err.Error()
	}
	if resp != nil && resp.Error != nil {
		return resp.Error.Message
	}
	return "unknown error"
}

func timeoutError(err error) bool {
	if os.IsTimeout(err) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	return false
}

func sdkFleetCircuitOpenByGroup(probes []sdkHealthProbe, unhealthyFleetFraction float64) map[string]bool {
	countsByGroup := map[string]sdkFleetCircuitCount{}
	for _, probe := range probes {
		group := sdkServerGroup(probe.server.Address)
		counts := countsByGroup[group]
		counts.total++
		if probe.result != sdkProbeHealthy && probe.result != sdkProbeStarting {
			counts.unhealthy++
		}
		countsByGroup[group] = counts
	}

	openByGroup := map[string]bool{}
	for group, counts := range countsByGroup {
		openByGroup[group] = sdkFleetCircuitOpen(counts.unhealthy, counts.total, unhealthyFleetFraction)
	}
	return openByGroup
}

func sdkFleetCircuitOpen(unhealthyCount int, total int, unhealthyFleetFraction float64) bool {
	if total == 0 {
		return false
	}
	if unhealthyCount < sdkFleetCircuitMinCount {
		return false
	}
	unhealthyFraction := float64(unhealthyCount) / float64(total)
	return unhealthyFraction > unhealthyFleetFraction
}

func sdkServerGroup(address string) string {
	host := address
	parsed, err := url.Parse(address)
	if err == nil && parsed.Host != "" {
		host = parsed.Host
	}
	splitHost, _, err := net.SplitHostPort(host)
	if err == nil {
		host = splitHost
	}
	host = strings.Split(host, ".")[0]
	if strings.HasPrefix(host, "lbrynet-") {
		group := strings.TrimPrefix(host, "lbrynet-")
		lastDash := strings.LastIndex(group, "-")
		if lastDash > 0 {
			_, err = strconv.Atoi(group[lastDash+1:])
			if err == nil {
				return group[:lastDash]
			}
		}
		if group != "" {
			return group
		}
	}
	return host
}

func (r *Router) markSDKHealthy(address string) *sdkActiveRecovery {
	r.healthMu.Lock()
	defer r.healthMu.Unlock()

	state := r.sdkHealthStateForAddress(address)
	state.consecutiveDeadLoops = 0
	recovery := cloneSDKRecovery(state.recovery)
	state.recovery = nil
	return recovery
}

func (r *Router) markSDKNonActionable(address string) {
	r.healthMu.Lock()
	defer r.healthMu.Unlock()

	state := r.sdkHealthStateForAddress(address)
	state.consecutiveDeadLoops = 0
}

func (r *Router) markSDKDeadLoop(address string, failureThreshold int) bool {
	r.healthMu.Lock()
	defer r.healthMu.Unlock()

	state := r.sdkHealthStateForAddress(address)
	state.consecutiveDeadLoops++
	return state.consecutiveDeadLoops >= failureThreshold
}

func (r *Router) currentSDKRecovery(address string) *sdkActiveRecovery {
	r.healthMu.Lock()
	defer r.healthMu.Unlock()

	state := r.sdkHealthStateForAddress(address)
	return cloneSDKRecovery(state.recovery)
}

func (r *Router) setSDKRecovery(address string, recovery *sdkActiveRecovery) {
	r.healthMu.Lock()
	defer r.healthMu.Unlock()

	state := r.sdkHealthStateForAddress(address)
	state.recovery = cloneSDKRecovery(recovery)
}

func (r *Router) clearSDKRecovery(address string, recovery *sdkActiveRecovery) bool {
	r.healthMu.Lock()
	defer r.healthMu.Unlock()

	state := r.sdkHealthStateForAddress(address)
	if state.recovery == nil {
		return false
	}
	if state.recovery.endpointKey != recovery.endpointKey || state.recovery.slotKey != recovery.slotKey {
		return false
	}
	state.recovery = nil
	return true
}

func (r *Router) sdkHealthStateForAddress(address string) *sdkHealthState {
	if r.healthState == nil {
		r.healthState = map[string]*sdkHealthState{}
	}
	state := r.healthState[address]
	if state == nil {
		state = &sdkHealthState{}
		r.healthState[address] = state
	}
	return state
}

func activeRecoveryFromLease(lease *sdkRecoveryLease, cfg sdkHealthRuntimeConfig) *sdkActiveRecovery {
	return &sdkActiveRecovery{
		endpointKey: lease.endpointKey,
		slotKey:     lease.slotKey,
		deadline:    time.Now().Add(cfg.ReconnectObservation),
		attempts:    lease.attempts,
	}
}

func cloneSDKRecovery(recovery *sdkActiveRecovery) *sdkActiveRecovery {
	if recovery == nil {
		return nil
	}
	copy := *recovery
	return &copy
}

func decisionMetric(address string, decision string) {
	if decision == "" {
		return
	}
	metrics.LbrynetWalletReconnectDecisionTotal.WithLabelValues(address, decision).Inc()
}

func newSDKHealthOwner() string {
	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		hostname = "unknown-host"
	}
	return fmt.Sprintf("%s:%d:%d", hostname, os.Getpid(), time.Now().UnixNano())
}

func reportSDKMaxAttemptsExhausted(address string, reason string) {
	err := fmt.Errorf("SDK reconnect attempts exhausted for %s: %s", address, reason)
	logger.Log().Error(err)
	monitor.ErrorToSentry(err, map[string]string{sdkMonitorContextKey: address})
	decisionMetric(address, sdkReconnectDecisionMaxAttemptsExhausted)
}

func (r *Router) reportSDKMaxAttemptsExhaustedOnce(store *sdkLeaseStore, address string, reason string) {
	shouldReport := true
	if store != nil {
		var err error
		shouldReport, err = store.claimMaxAttemptsAlert(address)
		if err != nil {
			logger.Log().Errorf("failed to record SDK max-attempt alert state for %s: %s", address, err)
		}
	}
	if shouldReport {
		reportSDKMaxAttemptsExhausted(address, reason)
		return
	}
	logger.Log().Warnf("SDK reconnect suppressed for %s: max attempts already reported", address)
	decisionMetric(address, sdkReconnectDecisionMaxAttemptsExhausted)
}
