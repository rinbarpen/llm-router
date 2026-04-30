package services

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/rinbarpen/llm-router/src/schemas"
)

const (
	ConsoleSessionTTL = 7 * 24 * time.Hour
)

var ErrUnauthorized = errors.New("unauthorized")
var ErrInsufficientBalance = errors.New("wallet balance insufficient")

func (s *CatalogService) ConsolePasswordLogin(ctx context.Context, email string, password string, remoteAddr string, userAgent string) (schemas.ConsoleSession, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" || strings.TrimSpace(password) == "" {
		return schemas.ConsoleSession{}, ErrUnauthorized
	}
	var (
		user schemas.User
		hash string
	)
	if err := s.pool.QueryRow(ctx, `
		SELECT u.id, u.email, u.display_name, u.status, u.is_platform_admin, u.created_at, u.updated_at, c.password_hash
		FROM users u
		JOIN user_password_credentials c ON c.user_id = u.id
		WHERE lower(u.email) = $1
	`, email).Scan(&user.ID, &user.Email, &user.DisplayName, &user.Status, new(bool), &user.CreatedAt, &user.UpdatedAt, &hash); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return schemas.ConsoleSession{}, ErrUnauthorized
		}
		return schemas.ConsoleSession{}, fmt.Errorf("console password login: %w", err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return schemas.ConsoleSession{}, ErrUnauthorized
	}
	sessionToken, err := generateConsoleToken()
	if err != nil {
		return schemas.ConsoleSession{}, fmt.Errorf("generate console session: %w", err)
	}
	expiresAt := time.Now().UTC().Add(ConsoleSessionTTL)
	if _, err := s.pool.Exec(ctx, `
		INSERT INTO console_sessions(session_token, user_id, expires_at, remote_addr, user_agent)
		VALUES ($1,$2,$3,$4,$5)
	`, sessionToken, user.ID, expiresAt, nullIfEmpty(remoteAddr), nullIfEmpty(userAgent)); err != nil {
		return schemas.ConsoleSession{}, fmt.Errorf("create console session: %w", err)
	}
	user.Roles = []string{"member"}
	if isPlatformAdmin, err := s.isPlatformAdmin(ctx, user.ID); err == nil && isPlatformAdmin {
		user.Roles = append([]string{"platform_admin"}, user.Roles...)
	}
	return schemas.ConsoleSession{
		Token:     sessionToken,
		ExpiresAt: &expiresAt,
		User:      user,
	}, nil
}

func (s *CatalogService) GetConsoleSession(ctx context.Context, token string) (schemas.ConsoleSession, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return schemas.ConsoleSession{}, ErrUnauthorized
	}
	var (
		session schemas.ConsoleSession
		admin   bool
	)
	if err := s.pool.QueryRow(ctx, `
		SELECT s.session_token, s.expires_at, u.id, u.email, u.display_name, u.status, u.is_platform_admin, u.created_at, u.updated_at
		FROM console_sessions s
		JOIN users u ON u.id = s.user_id
		WHERE s.session_token = $1 AND s.revoked_at IS NULL AND s.expires_at > now()
	`, token).Scan(&session.Token, &session.ExpiresAt, &session.User.ID, &session.User.Email, &session.User.DisplayName, &session.User.Status, &admin, &session.User.CreatedAt, &session.User.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return schemas.ConsoleSession{}, ErrUnauthorized
		}
		return schemas.ConsoleSession{}, fmt.Errorf("get console session: %w", err)
	}
	roles, err := s.listUserRoles(ctx, session.User.ID, admin)
	if err != nil {
		return schemas.ConsoleSession{}, err
	}
	session.User.Roles = roles
	_, _ = s.pool.Exec(ctx, `UPDATE console_sessions SET last_seen_at = now() WHERE session_token = $1`, token)
	return session, nil
}

func (s *CatalogService) DeleteConsoleSession(ctx context.Context, token string) error {
	_, err := s.pool.Exec(ctx, `UPDATE console_sessions SET revoked_at = now() WHERE session_token = $1`, strings.TrimSpace(token))
	if err != nil {
		return fmt.Errorf("delete console session: %w", err)
	}
	return nil
}

func (s *CatalogService) ListConsoleUsers(ctx context.Context) ([]schemas.User, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, email, display_name, status, is_platform_admin, created_at, updated_at
		FROM users
		ORDER BY id ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list console users: %w", err)
	}
	defer rows.Close()
	out := make([]schemas.User, 0)
	for rows.Next() {
		var item schemas.User
		var admin bool
		if err := rows.Scan(&item.ID, &item.Email, &item.DisplayName, &item.Status, &admin, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan console user: %w", err)
		}
		item.Roles, _ = s.listUserRoles(ctx, item.ID, admin)
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *CatalogService) UpdateConsoleUser(ctx context.Context, id int64, in schemas.UserUpdate) (schemas.User, error) {
	currentUsers, err := s.ListConsoleUsers(ctx)
	if err != nil {
		return schemas.User{}, err
	}
	var found *schemas.User
	for i := range currentUsers {
		if currentUsers[i].ID == id {
			found = &currentUsers[i]
			break
		}
	}
	if found == nil {
		return schemas.User{}, ErrNotFound
	}
	displayName := found.DisplayName
	if in.DisplayName != nil {
		displayName = strings.TrimSpace(*in.DisplayName)
	}
	status := found.Status
	if in.Status != nil && strings.TrimSpace(*in.Status) != "" {
		status = strings.TrimSpace(*in.Status)
	}
	if err := s.pool.QueryRow(ctx, `
		UPDATE users
		SET display_name = $2, status = $3, updated_at = now()
		WHERE id = $1
		RETURNING id, email, display_name, status, created_at, updated_at
	`, id, displayName, status).Scan(&found.ID, &found.Email, &found.DisplayName, &found.Status, &found.CreatedAt, &found.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return schemas.User{}, ErrNotFound
		}
		return schemas.User{}, fmt.Errorf("update console user: %w", err)
	}
	admin, err := s.isPlatformAdmin(ctx, found.ID)
	if err == nil {
		found.Roles, _ = s.listUserRoles(ctx, found.ID, admin)
	}
	return *found, nil
}

func (s *CatalogService) ListConsoleTeams(ctx context.Context) ([]schemas.Team, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, name, slug, status, description, created_at, updated_at
		FROM teams
		ORDER BY id ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list console teams: %w", err)
	}
	defer rows.Close()
	out := make([]schemas.Team, 0)
	for rows.Next() {
		var item schemas.Team
		if err := rows.Scan(&item.ID, &item.Name, &item.Slug, &item.Status, &item.Description, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan console team: %w", err)
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *CatalogService) CreateConsoleTeam(ctx context.Context, in schemas.TeamCreate, ownerUserID *int64) (schemas.Team, error) {
	var item schemas.Team
	if err := s.pool.QueryRow(ctx, `
		INSERT INTO teams(name, slug, description, owner_user_id)
		VALUES ($1,$2,$3,$4)
		RETURNING id, name, slug, status, description, created_at, updated_at
	`, strings.TrimSpace(in.Name), strings.TrimSpace(in.Slug), in.Description, ownerUserID).Scan(
		&item.ID, &item.Name, &item.Slug, &item.Status, &item.Description, &item.CreatedAt, &item.UpdatedAt,
	); err != nil {
		return schemas.Team{}, fmt.Errorf("create console team: %w", err)
	}
	if ownerUserID != nil && *ownerUserID > 0 {
		_, err := s.pool.Exec(ctx, `
			INSERT INTO team_members(team_id, user_id, role, status, invited_by_user_id)
			VALUES ($1,$2,'team_owner','active',$2)
			ON CONFLICT(team_id, user_id) DO UPDATE SET role = EXCLUDED.role, status = 'active', updated_at = now()
		`, item.ID, *ownerUserID)
		if err != nil {
			return schemas.Team{}, fmt.Errorf("create owner membership: %w", err)
		}
	}
	return item, nil
}

func (s *CatalogService) ListTeamMembers(ctx context.Context, teamID int64) ([]schemas.TeamMember, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT tm.id, tm.team_id, tm.user_id, u.email, u.display_name, tm.role, tm.status, tm.created_at, tm.updated_at
		FROM team_members tm
		JOIN users u ON u.id = tm.user_id
		WHERE tm.team_id = $1
		ORDER BY tm.id ASC
	`, teamID)
	if err != nil {
		return nil, fmt.Errorf("list team members: %w", err)
	}
	defer rows.Close()
	out := make([]schemas.TeamMember, 0)
	for rows.Next() {
		var item schemas.TeamMember
		if err := rows.Scan(&item.ID, &item.TeamID, &item.UserID, &item.UserEmail, &item.DisplayName, &item.Role, &item.Status, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan team member: %w", err)
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *CatalogService) AddTeamMember(ctx context.Context, teamID int64, in schemas.TeamMemberCreate, invitedByUserID *int64) (schemas.TeamMember, error) {
	role := strings.TrimSpace(in.Role)
	if role == "" {
		role = "member"
	}
	var item schemas.TeamMember
	if err := s.pool.QueryRow(ctx, `
		INSERT INTO team_members(team_id, user_id, role, status, invited_by_user_id)
		SELECT $1, u.id, $3, 'active', $4
		FROM users u
		WHERE u.id = $2
		ON CONFLICT(team_id, user_id) DO UPDATE SET
			role = EXCLUDED.role,
			status = 'active',
			invited_by_user_id = EXCLUDED.invited_by_user_id,
			updated_at = now()
		RETURNING team_members.id, team_members.team_id, team_members.user_id, team_members.role, team_members.status, team_members.created_at, team_members.updated_at
	`, teamID, in.UserID, role, invitedByUserID).Scan(&item.ID, &item.TeamID, &item.UserID, &item.Role, &item.Status, &item.CreatedAt, &item.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return schemas.TeamMember{}, ErrNotFound
		}
		return schemas.TeamMember{}, fmt.Errorf("add team member: %w", err)
	}
	if err := s.pool.QueryRow(ctx, `SELECT email, display_name FROM users WHERE id = $1`, item.UserID).Scan(&item.UserEmail, &item.DisplayName); err != nil {
		return schemas.TeamMember{}, fmt.Errorf("load team member identity: %w", err)
	}
	return item, nil
}

func (s *CatalogService) UpdateTeamMember(ctx context.Context, teamID int64, userID int64, in schemas.TeamMemberUpdate) (schemas.TeamMember, error) {
	members, err := s.ListTeamMembers(ctx, teamID)
	if err != nil {
		return schemas.TeamMember{}, err
	}
	var current *schemas.TeamMember
	for i := range members {
		if members[i].UserID == userID {
			current = &members[i]
			break
		}
	}
	if current == nil {
		return schemas.TeamMember{}, ErrNotFound
	}
	role := current.Role
	if in.Role != nil && strings.TrimSpace(*in.Role) != "" {
		role = strings.TrimSpace(*in.Role)
	}
	status := current.Status
	if in.Status != nil && strings.TrimSpace(*in.Status) != "" {
		status = strings.TrimSpace(*in.Status)
	}
	if err := s.pool.QueryRow(ctx, `
		UPDATE team_members tm
		SET role = $3, status = $4, updated_at = now()
		FROM users u
		WHERE tm.team_id = $1 AND tm.user_id = $2 AND u.id = tm.user_id
		RETURNING tm.id, tm.team_id, tm.user_id, u.email, u.display_name, tm.role, tm.status, tm.created_at, tm.updated_at
	`, teamID, userID, role, status).Scan(
		&current.ID, &current.TeamID, &current.UserID, &current.UserEmail, &current.DisplayName, &current.Role, &current.Status, &current.CreatedAt, &current.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return schemas.TeamMember{}, ErrNotFound
		}
		return schemas.TeamMember{}, fmt.Errorf("update team member: %w", err)
	}
	return *current, nil
}

func (s *CatalogService) CreateTeamInvite(ctx context.Context, teamID int64, in schemas.TeamInviteCreate, invitedByUserID *int64) (schemas.TeamInvite, error) {
	email := strings.ToLower(strings.TrimSpace(in.Email))
	if email == "" {
		return schemas.TeamInvite{}, fmt.Errorf("email is required")
	}
	role := strings.TrimSpace(in.Role)
	if role == "" {
		role = "member"
	}
	token, err := generateInviteToken()
	if err != nil {
		return schemas.TeamInvite{}, fmt.Errorf("generate invite token: %w", err)
	}
	expiresAt := time.Now().UTC().Add(7 * 24 * time.Hour)
	var item schemas.TeamInvite
	if err := s.pool.QueryRow(ctx, `
		INSERT INTO team_invites(team_id, email, role, invite_token, status, expires_at, invited_by_user_id)
		VALUES ($1,$2,$3,$4,'pending',$5,$6)
		RETURNING id, team_id, email, role, invite_token, status, expires_at, created_at, updated_at
	`, teamID, email, role, token, expiresAt, invitedByUserID).Scan(
		&item.ID, &item.TeamID, &item.Email, &item.Role, &item.InviteToken, &item.Status, &item.ExpiresAt, &item.CreatedAt, &item.UpdatedAt,
	); err != nil {
		return schemas.TeamInvite{}, fmt.Errorf("create team invite: %w", err)
	}
	return item, nil
}

func (s *CatalogService) ListTeamInvites(ctx context.Context, teamID int64) ([]schemas.TeamInvite, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, team_id, email, role, invite_token, status, expires_at, created_at, updated_at
		FROM team_invites
		WHERE team_id = $1
		ORDER BY id DESC
	`, teamID)
	if err != nil {
		return nil, fmt.Errorf("list team invites: %w", err)
	}
	defer rows.Close()
	out := make([]schemas.TeamInvite, 0)
	for rows.Next() {
		var item schemas.TeamInvite
		if err := rows.Scan(&item.ID, &item.TeamID, &item.Email, &item.Role, &item.InviteToken, &item.Status, &item.ExpiresAt, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan team invite: %w", err)
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *CatalogService) AcceptTeamInvite(ctx context.Context, inviteToken string, userID int64) (schemas.TeamMember, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return schemas.TeamMember{}, fmt.Errorf("begin accept invite tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var (
		inviteID int64
		teamID   int64
		role     string
		status   string
		expires  time.Time
	)
	if err := tx.QueryRow(ctx, `
		SELECT id, team_id, role, status, expires_at
		FROM team_invites
		WHERE invite_token = $1
		FOR UPDATE
	`, strings.TrimSpace(inviteToken)).Scan(&inviteID, &teamID, &role, &status, &expires); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return schemas.TeamMember{}, ErrNotFound
		}
		return schemas.TeamMember{}, fmt.Errorf("load team invite: %w", err)
	}
	if status != "pending" || time.Now().UTC().After(expires) {
		return schemas.TeamMember{}, fmt.Errorf("invite is no longer valid")
	}

	var member schemas.TeamMember
	if err := tx.QueryRow(ctx, `
		INSERT INTO team_members(team_id, user_id, role, status)
		VALUES ($1,$2,$3,'active')
		ON CONFLICT(team_id, user_id) DO UPDATE SET role = EXCLUDED.role, status = 'active', updated_at = now()
		RETURNING id, team_id, user_id, role, status, created_at, updated_at
	`, teamID, userID, role).Scan(&member.ID, &member.TeamID, &member.UserID, &member.Role, &member.Status, &member.CreatedAt, &member.UpdatedAt); err != nil {
		return schemas.TeamMember{}, fmt.Errorf("upsert accepted team member: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE team_invites SET status = 'accepted', updated_at = now() WHERE id = $1`, inviteID); err != nil {
		return schemas.TeamMember{}, fmt.Errorf("mark invite accepted: %w", err)
	}
	if err := tx.QueryRow(ctx, `SELECT email, display_name FROM users WHERE id = $1`, userID).Scan(&member.UserEmail, &member.DisplayName); err != nil {
		return schemas.TeamMember{}, fmt.Errorf("load accepted member identity: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return schemas.TeamMember{}, fmt.Errorf("commit accept invite: %w", err)
	}
	return member, nil
}

func (s *CatalogService) GetWalletSummary(ctx context.Context, ownerType string, ownerID int64) (schemas.Wallet, error) {
	var item schemas.Wallet
	if err := s.pool.QueryRow(ctx, `
		INSERT INTO wallets(owner_type, owner_id)
		VALUES ($1,$2)
		ON CONFLICT(owner_type, owner_id) DO UPDATE SET updated_at = wallets.updated_at
		RETURNING id, owner_type, owner_id, currency, balance::float8, status, created_at, updated_at
	`, ownerType, ownerID).Scan(&item.ID, &item.OwnerType, &item.OwnerID, &item.Currency, &item.Balance, &item.Status, &item.CreatedAt, &item.UpdatedAt); err != nil {
		return schemas.Wallet{}, fmt.Errorf("get wallet summary: %w", err)
	}
	return item, nil
}

func (s *CatalogService) ListRechargeOrders(ctx context.Context, ownerType string, ownerID int64) ([]schemas.RechargeOrder, error) {
	query := `
		SELECT id, order_no, owner_type, owner_id, amount::float8, currency, status, payment_provider, created_at, updated_at
		FROM recharge_orders
	`
	args := []any{}
	if ownerType != "" && ownerID > 0 {
		query += ` WHERE owner_type = $1 AND owner_id = $2`
		args = append(args, ownerType, ownerID)
	}
	query += ` ORDER BY id DESC`
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list recharge orders: %w", err)
	}
	defer rows.Close()
	out := make([]schemas.RechargeOrder, 0)
	for rows.Next() {
		var item schemas.RechargeOrder
		if err := rows.Scan(&item.ID, &item.OrderNo, &item.OwnerType, &item.OwnerID, &item.Amount, &item.Currency, &item.Status, &item.PaymentProvider, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan recharge order: %w", err)
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *CatalogService) CheckAPIKeyWallet(ctx context.Context, item schemas.APIKey) error {
	ownerType := strings.TrimSpace(item.OwnerType)
	if ownerType == "" || ownerType == "system" || item.OwnerID == nil {
		return nil
	}
	wallet, err := s.GetWalletSummary(ctx, ownerType, *item.OwnerID)
	if err != nil {
		return err
	}
	if wallet.Balance <= 0 {
		return ErrInsufficientBalance
	}
	return nil
}

func (s *CatalogService) listUserRoles(ctx context.Context, userID int64, admin bool) ([]string, error) {
	roles := make([]string, 0, 4)
	if admin {
		roles = append(roles, "platform_admin")
	}
	rows, err := s.pool.Query(ctx, `SELECT DISTINCT role FROM team_members WHERE user_id = $1 AND status = 'active' ORDER BY role ASC`, userID)
	if err != nil {
		return roles, fmt.Errorf("list user roles: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var role string
		if err := rows.Scan(&role); err != nil {
			return nil, fmt.Errorf("scan user role: %w", err)
		}
		roles = append(roles, role)
	}
	if len(roles) == 0 {
		roles = append(roles, "member")
	}
	return roles, nil
}

func (s *CatalogService) isPlatformAdmin(ctx context.Context, userID int64) (bool, error) {
	var admin bool
	if err := s.pool.QueryRow(ctx, `SELECT is_platform_admin FROM users WHERE id = $1`, userID).Scan(&admin); err != nil {
		return false, fmt.Errorf("check platform admin: %w", err)
	}
	return admin, nil
}

func generateConsoleToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return "cs_" + base64.RawURLEncoding.EncodeToString(buf), nil
}

func generateInviteToken() (string, error) {
	var buf [12]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return "ti_" + hex.EncodeToString(buf[:]), nil
}

func nullIfEmpty(v string) any {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	return strings.TrimSpace(v)
}
