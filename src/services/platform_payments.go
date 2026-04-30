package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/rinbarpen/llm-router/src/db"
	"github.com/rinbarpen/llm-router/src/schemas"
)

func (s *CatalogService) CreateRechargeOrder(ctx context.Context, ownerType string, ownerID int64, amount float64, currency string, provider string, createdByUserID *int64) (schemas.RechargeOrder, map[string]any, error) {
	if ownerID <= 0 || amount <= 0 {
		return schemas.RechargeOrder{}, nil, fmt.Errorf("invalid recharge order params")
	}
	if strings.TrimSpace(currency) == "" {
		currency = "CNY"
	}
	if strings.TrimSpace(provider) == "" {
		provider = "stripe"
	}
	wallet, err := s.GetWalletSummary(ctx, ownerType, ownerID)
	if err != nil {
		return schemas.RechargeOrder{}, nil, err
	}
	orderNo := "RO-" + strconv.FormatInt(time.Now().UTC().UnixNano(), 10)
	var order schemas.RechargeOrder
	if err := s.pool.QueryRow(ctx, `
		INSERT INTO recharge_orders(order_no, owner_type, owner_id, wallet_id, amount, currency, status, payment_provider, subject, created_by_user_id)
		VALUES ($1,$2,$3,$4,$5,$6,'pending',$7,$8,$9)
		RETURNING id, order_no, owner_type, owner_id, amount::float8, currency, status, payment_provider, created_at, updated_at
	`, orderNo, ownerType, ownerID, wallet.ID, amount, currency, provider, "Wallet recharge", createdByUserID).Scan(
		&order.ID, &order.OrderNo, &order.OwnerType, &order.OwnerID, &order.Amount, &order.Currency, &order.Status, &order.PaymentProvider, &order.CreatedAt, &order.UpdatedAt,
	); err != nil {
		return schemas.RechargeOrder{}, nil, fmt.Errorf("create recharge order: %w", err)
	}
	checkout := map[string]any{
		"provider":     provider,
		"order_no":     orderNo,
		"payment_url":  fmt.Sprintf("/pay/%s/%s", provider, orderNo),
		"qr_code_text": fmt.Sprintf("%s:%s", provider, orderNo),
	}
	return order, checkout, nil
}

func (s *CatalogService) GetRechargeOrder(ctx context.Context, orderNo string) (schemas.RechargeOrder, error) {
	var order schemas.RechargeOrder
	if err := s.pool.QueryRow(ctx, `
		SELECT id, order_no, owner_type, owner_id, amount::float8, currency, status, payment_provider, created_at, updated_at
		FROM recharge_orders
		WHERE order_no = $1
	`, strings.TrimSpace(orderNo)).Scan(
		&order.ID, &order.OrderNo, &order.OwnerType, &order.OwnerID, &order.Amount, &order.Currency, &order.Status, &order.PaymentProvider, &order.CreatedAt, &order.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return schemas.RechargeOrder{}, ErrNotFound
		}
		return schemas.RechargeOrder{}, fmt.Errorf("get recharge order: %w", err)
	}
	return order, nil
}

func (s *CatalogService) MarkRechargeOrderPaid(ctx context.Context, provider string, eventID string, orderNo string, providerTradeNo string, payload map[string]any) (schemas.RechargeOrder, bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return schemas.RechargeOrder{}, false, fmt.Errorf("begin payment callback tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	payloadRaw, _ := json.Marshal(payload)
	if _, err := tx.Exec(ctx, `
		INSERT INTO payment_callbacks(provider, event_id, order_no, payload, processed_at)
		VALUES ($1,$2,$3,$4,now())
		ON CONFLICT(provider, event_id) DO NOTHING
	`, provider, eventID, orderNo, payloadRaw); err != nil {
		return schemas.RechargeOrder{}, false, fmt.Errorf("insert payment callback: %w", err)
	}
	var (
		order    schemas.RechargeOrder
		walletID int64
	)
	if err := tx.QueryRow(ctx, `
		SELECT id, order_no, owner_type, owner_id, wallet_id, amount::float8, currency, status, payment_provider, created_at, updated_at
		FROM recharge_orders
		WHERE order_no = $1
		FOR UPDATE
	`, orderNo).Scan(
		&order.ID, &order.OrderNo, &order.OwnerType, &order.OwnerID, &walletID, &order.Amount, &order.Currency, &order.Status, &order.PaymentProvider, &order.CreatedAt, &order.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return schemas.RechargeOrder{}, false, ErrNotFound
		}
		return schemas.RechargeOrder{}, false, fmt.Errorf("lock recharge order: %w", err)
	}
	if strings.EqualFold(order.Status, "paid") {
		if err := tx.Commit(ctx); err != nil {
			return schemas.RechargeOrder{}, false, fmt.Errorf("commit duplicate callback: %w", err)
		}
		return order, false, nil
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO payment_attempts(order_id, provider, provider_trade_no, status, request_payload, response_payload)
		VALUES ($1,$2,$3,'succeeded',$4,$5)
	`, order.ID, provider, nullIfEmpty(providerTradeNo), payloadRaw, payloadRaw); err != nil {
		return schemas.RechargeOrder{}, false, fmt.Errorf("insert payment attempt: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE recharge_orders
		SET status = 'paid', paid_at = now(), updated_at = now()
		WHERE id = $1
	`, order.ID); err != nil {
		return schemas.RechargeOrder{}, false, fmt.Errorf("mark order paid: %w", err)
	}
	if err := s.applyWalletDeltaTx(ctx, tx, walletID, order.Amount, order.Currency, "recharge", "recharge_order", order.OrderNo, map[string]any{
		"payment_provider": provider,
		"event_id":         eventID,
	}); err != nil {
		return schemas.RechargeOrder{}, false, err
	}
	order.Status = "paid"
	now := time.Now().UTC()
	order.UpdatedAt = &now
	if err := tx.Commit(ctx); err != nil {
		return schemas.RechargeOrder{}, false, fmt.Errorf("commit payment callback: %w", err)
	}
	return order, true, nil
}

func (s *CatalogService) applyWalletDebitForAPIKey(ctx context.Context, apiKeyID int64, cost float64, tokens int64) error {
	if cost <= 0 {
		return nil
	}
	item, err := s.GetAPIKey(ctx, apiKeyID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(item.OwnerType) == "" || item.OwnerType == "system" || item.OwnerID == nil {
		return nil
	}
	wallet, err := s.GetWalletSummary(ctx, item.OwnerType, *item.OwnerID)
	if err != nil {
		return err
	}
	return s.applyWalletDelta(ctx, wallet.ID, -cost, wallet.Currency, "usage_debit", "api_key", strconv.FormatInt(apiKeyID, 10), map[string]any{
		"tokens": tokens,
		"cost":   cost,
	})
}

func (s *CatalogService) applyWalletDelta(ctx context.Context, walletID int64, amount float64, currency string, reason string, refType string, refID string, metadata map[string]any) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin wallet delta tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := s.applyWalletDeltaTx(ctx, tx, walletID, amount, currency, reason, refType, refID, metadata); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit wallet delta: %w", err)
	}
	return nil
}

func (s *CatalogService) applyWalletDeltaTx(ctx context.Context, tx *db.Tx, walletID int64, amount float64, currency string, reason string, refType string, refID string, metadata map[string]any) error {
	var before float64
	if err := tx.QueryRow(ctx, `SELECT balance::float8 FROM wallets WHERE id = $1 FOR UPDATE`, walletID).Scan(&before); err != nil {
		return fmt.Errorf("lock wallet: %w", err)
	}
	after := before + amount
	if after < 0 {
		return ErrInsufficientBalance
	}
	if _, err := tx.Exec(ctx, `UPDATE wallets SET balance = $2, updated_at = now() WHERE id = $1`, walletID, after); err != nil {
		return fmt.Errorf("update wallet balance: %w", err)
	}
	metaRaw, _ := json.Marshal(metadata)
	if _, err := tx.Exec(ctx, `
		INSERT INTO wallet_ledger_entries(wallet_id, entry_type, amount, balance_before, balance_after, currency, reason, reference_type, reference_id, metadata)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
	`, walletID, ledgerEntryType(amount), amount, before, after, currency, reason, nullIfEmpty(refType), nullIfEmpty(refID), metaRaw); err != nil {
		return fmt.Errorf("insert wallet ledger: %w", err)
	}
	return nil
}

func ledgerEntryType(amount float64) string {
	if amount >= 0 {
		return "credit"
	}
	return "debit"
}
