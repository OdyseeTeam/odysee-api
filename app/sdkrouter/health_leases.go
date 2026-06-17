package sdkrouter

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/OdyseeTeam/odysee-api/internal/storage"
	"github.com/OdyseeTeam/odysee-api/models"

	"github.com/volatiletech/null"
	"github.com/volatiletech/sqlboiler/boil"
	"github.com/volatiletech/sqlboiler/queries/qm"
)

const (
	sdkLeaseKindEndpoint   = "endpoint"
	sdkLeaseKindGlobalSlot = "global_slot"
	sdkEndpointLeasePrefix = "endpoint:"
	sdkGlobalLeasePrefix   = "global:wallet_reconnect:"
)

type sdkLeaseStore struct {
	db    *sql.DB
	owner string
}

type sdkRecoveryLease struct {
	endpointKey string
	slotKey     string
	attempts    int
}

func newSDKLeaseStore(owner string) (*sdkLeaseStore, error) {
	if storage.DB == nil {
		return nil, fmt.Errorf("storage DB is not initialized")
	}
	return &sdkLeaseStore{db: storage.DB, owner: owner}, nil
}

func (s *sdkLeaseStore) acquire(endpointAddress string, cfg sdkHealthRuntimeConfig) (*sdkRecoveryLease, string, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, "", err
	}

	committed := false
	defer rollbackUnlessCommitted(tx, &committed)

	now, err := models.SDKReconnectLeaseDBNow(tx)
	if err != nil {
		return nil, "", err
	}

	endpointKey := sdkEndpointLeaseKey(endpointAddress)
	endpointLease, err := getOrCreateSDKLease(tx, endpointKey, sdkLeaseKindEndpoint, now)
	if err != nil {
		return nil, "", err
	}
	if !s.leaseAvailable(endpointLease, now) {
		return nil, sdkReconnectDecisionLeaseUnavailable, nil
	}
	if endpointLease.Attempts >= cfg.ReconnectMaxAttempts {
		return nil, sdkReconnectDecisionMaxAttemptsExhausted, nil
	}
	if endpointLease.LastReconnectAt.Valid && endpointLease.LastReconnectAt.Time.Add(cfg.ReconnectCooldown).After(now) {
		return nil, sdkReconnectDecisionCooldown, nil
	}

	var slotLease *models.SDKReconnectLease
	for slot := 0; slot < cfg.ReconnectMaxConcurrent; slot++ {
		key := sdkGlobalLeaseKey(slot)
		lease, err := getOrCreateSDKLease(tx, key, sdkLeaseKindGlobalSlot, now)
		if err != nil {
			return nil, "", err
		}
		if !s.leaseAvailable(lease, now) {
			continue
		}
		slotLease = lease
		break
	}
	if slotLease == nil {
		return nil, sdkReconnectDecisionLeaseUnavailable, nil
	}

	expiresAt := now.Add(cfg.ReconnectLeaseTTL)
	endpointLease.Owner = null.StringFrom(s.owner)
	endpointLease.LeaseExpiresAt = null.TimeFrom(expiresAt)
	endpointLease.LastReconnectAt = null.TimeFrom(now)
	endpointLease.Attempts++
	err = updateSDKLease(tx, endpointLease, boil.Whitelist(
		models.SDKReconnectLeaseColumns.Owner,
		models.SDKReconnectLeaseColumns.LeaseExpiresAt,
		models.SDKReconnectLeaseColumns.LastReconnectAt,
		models.SDKReconnectLeaseColumns.Attempts,
		models.SDKReconnectLeaseColumns.UpdatedAt,
	))
	if err != nil {
		return nil, "", err
	}

	slotLease.Owner = null.StringFrom(s.owner)
	slotLease.LeaseExpiresAt = null.TimeFrom(expiresAt)
	err = updateSDKLease(tx, slotLease, boil.Whitelist(
		models.SDKReconnectLeaseColumns.Owner,
		models.SDKReconnectLeaseColumns.LeaseExpiresAt,
		models.SDKReconnectLeaseColumns.UpdatedAt,
	))
	if err != nil {
		return nil, "", err
	}

	err = tx.Commit()
	if err != nil {
		return nil, "", err
	}
	committed = true

	return &sdkRecoveryLease{endpointKey: endpointKey, slotKey: slotLease.LeaseKey, attempts: endpointLease.Attempts}, "", nil
}

func (s *sdkLeaseStore) renew(recovery *sdkActiveRecovery, cfg sdkHealthRuntimeConfig) error {
	err := validateGlobalLeaseKey(recovery.slotKey, cfg.ReconnectMaxConcurrent)
	if err != nil {
		return err
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}

	committed := false
	defer rollbackUnlessCommitted(tx, &committed)

	now, err := models.SDKReconnectLeaseDBNow(tx)
	if err != nil {
		return err
	}
	expiresAt := now.Add(cfg.ReconnectLeaseTTL)

	endpointLease, err := getSDKLeaseForUpdate(tx, recovery.endpointKey)
	if err != nil {
		return err
	}
	if s.owned(endpointLease) {
		endpointLease.LeaseExpiresAt = null.TimeFrom(expiresAt)
		err = updateSDKLease(tx, endpointLease, boil.Whitelist(
			models.SDKReconnectLeaseColumns.LeaseExpiresAt,
			models.SDKReconnectLeaseColumns.UpdatedAt,
		))
		if err != nil {
			return err
		}
	}

	slotLease, err := getSDKLeaseForUpdate(tx, recovery.slotKey)
	if err != nil {
		return err
	}
	if s.owned(slotLease) {
		slotLease.LeaseExpiresAt = null.TimeFrom(expiresAt)
		err = updateSDKLease(tx, slotLease, boil.Whitelist(
			models.SDKReconnectLeaseColumns.LeaseExpiresAt,
			models.SDKReconnectLeaseColumns.UpdatedAt,
		))
		if err != nil {
			return err
		}
	}

	err = tx.Commit()
	if err != nil {
		return err
	}
	committed = true
	return nil
}

func (s *sdkLeaseStore) release(recovery *sdkActiveRecovery, resetAttempts bool) error {
	err := validateGlobalLeaseKey(recovery.slotKey, 0)
	if err != nil {
		return err
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}

	committed := false
	defer rollbackUnlessCommitted(tx, &committed)

	endpointLease, err := getSDKLeaseForUpdate(tx, recovery.endpointKey)
	if err != nil {
		return err
	}
	if s.owned(endpointLease) {
		endpointLease.Owner = null.NewString("", false)
		endpointLease.LeaseExpiresAt = null.NewTime(time.Time{}, false)
		if resetAttempts {
			endpointLease.Attempts = 0
			endpointLease.MaxAttemptsAlertedAt = null.NewTime(time.Time{}, false)
		}
		err = updateSDKLease(tx, endpointLease, boil.Whitelist(
			models.SDKReconnectLeaseColumns.Owner,
			models.SDKReconnectLeaseColumns.LeaseExpiresAt,
			models.SDKReconnectLeaseColumns.Attempts,
			models.SDKReconnectLeaseColumns.MaxAttemptsAlertedAt,
			models.SDKReconnectLeaseColumns.UpdatedAt,
		))
		if err != nil {
			return err
		}
	}

	slotLease, err := getSDKLeaseForUpdate(tx, recovery.slotKey)
	if err != nil {
		return err
	}
	if s.owned(slotLease) {
		slotLease.Owner = null.NewString("", false)
		slotLease.LeaseExpiresAt = null.NewTime(time.Time{}, false)
		err = updateSDKLease(tx, slotLease, boil.Whitelist(
			models.SDKReconnectLeaseColumns.Owner,
			models.SDKReconnectLeaseColumns.LeaseExpiresAt,
			models.SDKReconnectLeaseColumns.UpdatedAt,
		))
		if err != nil {
			return err
		}
	}

	err = tx.Commit()
	if err != nil {
		return err
	}
	committed = true
	return nil
}

func (s *sdkLeaseStore) confirmHealthy(endpointAddress string, recovery *sdkActiveRecovery) error {
	if recovery != nil {
		err := validateGlobalLeaseKey(recovery.slotKey, 0)
		if err != nil {
			return err
		}
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}

	committed := false
	defer rollbackUnlessCommitted(tx, &committed)

	now, err := models.SDKReconnectLeaseDBNow(tx)
	if err != nil {
		return err
	}

	endpointLease, err := getSDKLeaseForUpdate(tx, sdkEndpointLeaseKey(endpointAddress))
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	if err == nil && (s.owned(endpointLease) || !endpointLease.LeaseExpiresAt.Valid || !endpointLease.LeaseExpiresAt.Time.After(now)) {
		endpointLease.Attempts = 0
		endpointLease.MaxAttemptsAlertedAt = null.NewTime(time.Time{}, false)
		endpointLease.Owner = null.NewString("", false)
		endpointLease.LeaseExpiresAt = null.NewTime(time.Time{}, false)
		err = updateSDKLease(tx, endpointLease, boil.Whitelist(
			models.SDKReconnectLeaseColumns.Owner,
			models.SDKReconnectLeaseColumns.LeaseExpiresAt,
			models.SDKReconnectLeaseColumns.Attempts,
			models.SDKReconnectLeaseColumns.MaxAttemptsAlertedAt,
			models.SDKReconnectLeaseColumns.UpdatedAt,
		))
		if err != nil {
			return err
		}
	}

	if recovery != nil {
		slotLease, err := getSDKLeaseForUpdate(tx, recovery.slotKey)
		if err != nil && err != sql.ErrNoRows {
			return err
		}
		if err == nil && s.owned(slotLease) {
			slotLease.Owner = null.NewString("", false)
			slotLease.LeaseExpiresAt = null.NewTime(time.Time{}, false)
			err = updateSDKLease(tx, slotLease, boil.Whitelist(
				models.SDKReconnectLeaseColumns.Owner,
				models.SDKReconnectLeaseColumns.LeaseExpiresAt,
				models.SDKReconnectLeaseColumns.UpdatedAt,
			))
			if err != nil {
				return err
			}
		}
	}

	err = tx.Commit()
	if err != nil {
		return err
	}
	committed = true
	return nil
}

func (s *sdkLeaseStore) claimMaxAttemptsAlert(endpointAddress string) (bool, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return false, err
	}

	committed := false
	defer rollbackUnlessCommitted(tx, &committed)

	now, err := models.SDKReconnectLeaseDBNow(tx)
	if err != nil {
		return false, err
	}

	endpointLease, err := getSDKLeaseForUpdate(tx, sdkEndpointLeaseKey(endpointAddress))
	if err == sql.ErrNoRows {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	if endpointLease.MaxAttemptsAlertedAt.Valid {
		err = tx.Commit()
		if err != nil {
			return false, err
		}
		committed = true
		return false, nil
	}

	endpointLease.MaxAttemptsAlertedAt = null.TimeFrom(now)
	err = updateSDKLease(tx, endpointLease, boil.Whitelist(
		models.SDKReconnectLeaseColumns.MaxAttemptsAlertedAt,
		models.SDKReconnectLeaseColumns.UpdatedAt,
	))
	if err != nil {
		return false, err
	}

	err = tx.Commit()
	if err != nil {
		return false, err
	}
	committed = true
	return true, nil
}

func (s *sdkLeaseStore) leaseAvailable(lease *models.SDKReconnectLease, now time.Time) bool {
	if !lease.LeaseExpiresAt.Valid || !lease.LeaseExpiresAt.Time.After(now) {
		return true
	}
	return s.owned(lease)
}

func (s *sdkLeaseStore) owned(lease *models.SDKReconnectLease) bool {
	return lease.Owner.Valid && lease.Owner.String == s.owner
}

func getSDKLeaseForUpdate(tx *sql.Tx, key string) (*models.SDKReconnectLease, error) {
	return models.SDKReconnectLeases(
		models.SDKReconnectLeaseWhere.LeaseKey.EQ(key),
		qm.For("UPDATE"),
	).One(tx)
}

func getOrCreateSDKLease(tx *sql.Tx, key string, kind string, now time.Time) (*models.SDKReconnectLease, error) {
	lease, err := getSDKLeaseForUpdate(tx, key)
	if err == nil {
		return validateSDKLeaseKind(lease, kind)
	}
	if err != sql.ErrNoRows {
		return nil, err
	}

	lease = &models.SDKReconnectLease{
		LeaseKey:  key,
		LeaseKind: kind,
		UpdatedAt: now,
	}
	err = lease.Upsert(tx, false, []string{models.SDKReconnectLeaseColumns.LeaseKey}, boil.Whitelist(), boil.Infer())
	if err != nil {
		return nil, err
	}

	lease, err = getSDKLeaseForUpdate(tx, key)
	if err != nil {
		return nil, err
	}
	return validateSDKLeaseKind(lease, kind)
}

func validateSDKLeaseKind(lease *models.SDKReconnectLease, kind string) (*models.SDKReconnectLease, error) {
	if lease.LeaseKind != kind {
		return nil, fmt.Errorf("sdk reconnect lease %s has kind %s, expected %s", lease.LeaseKey, lease.LeaseKind, kind)
	}
	return lease, nil
}

func updateSDKLease(tx *sql.Tx, lease *models.SDKReconnectLease, columns boil.Columns) error {
	_, err := lease.Update(tx, columns)
	return err
}

func rollbackUnlessCommitted(tx *sql.Tx, committed *bool) {
	if !*committed {
		_ = tx.Rollback()
	}
}

func sdkEndpointLeaseKey(address string) string {
	return sdkEndpointLeasePrefix + address
}

func sdkGlobalLeaseKey(slot int) string {
	return fmt.Sprintf("%s%d", sdkGlobalLeasePrefix, slot)
}

func validateGlobalLeaseKey(key string, maxSlots int) error {
	if !strings.HasPrefix(key, sdkGlobalLeasePrefix) {
		return fmt.Errorf("invalid sdk reconnect global lease key: %s", key)
	}
	slotText := strings.TrimPrefix(key, sdkGlobalLeasePrefix)
	slot, err := strconv.Atoi(slotText)
	if err != nil || slot < 0 {
		return fmt.Errorf("invalid sdk reconnect global lease key: %s", key)
	}
	if maxSlots > 0 && slot >= maxSlots {
		return fmt.Errorf("sdk reconnect global lease slot %d exceeds configured max %d", slot, maxSlots)
	}
	return nil
}
