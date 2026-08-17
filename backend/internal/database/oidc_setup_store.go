package database

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/KaeruApps/core/internal/identity"
	"github.com/KaeruApps/core/internal/installation"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type OIDCSetupStore struct {
	pool *pgxpool.Pool
}

func (store *OIDCSetupStore) ConsumeOIDCLoginAttempt(
	ctx context.Context,
	stateHash [sha256.Size]byte,
	now time.Time,
) (installation.OIDCConfiguration, installation.OIDCLoginAttempt, error) {
	transaction, err := store.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return installation.OIDCConfiguration{}, installation.OIDCLoginAttempt{}, fmt.Errorf("begin OIDC callback transaction: %w", err)
	}
	defer func() { _ = transaction.Rollback(ctx) }()

	attempt := installation.OIDCLoginAttempt{StateHash: stateHash}
	err = transaction.QueryRow(ctx, `
		DELETE FROM oidc_login_attempts
		WHERE state_hash = $1 AND expires_at > $2
		RETURNING code_verifier, nonce, redirect_uri, expires_at, created_at, purpose
	`, stateHash[:], now).Scan(
		&attempt.CodeVerifier,
		&attempt.Nonce,
		&attempt.RedirectURI,
		&attempt.ExpiresAt,
		&attempt.CreatedAt,
		&attempt.Purpose,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return installation.OIDCConfiguration{}, installation.OIDCLoginAttempt{}, identity.ErrLoginAttemptInvalid
	}
	if err != nil {
		return installation.OIDCConfiguration{}, installation.OIDCLoginAttempt{}, fmt.Errorf("consume OIDC login attempt: %w", err)
	}

	configuration := installation.OIDCConfiguration{}
	configurationQuery := `
		SELECT
			settings.provider_name,
			settings.issuer_url,
			settings.client_id,
			settings.client_secret,
			settings.additional_scopes,
			settings.username_claim,
			settings.display_name_claim,
			settings.avatar_claim,
			settings.groups_claim,
			settings.access_urls,
			settings.button_text,
			settings.redirect_uri,
			settings.updated_at
		FROM oidc_settings settings
		JOIN installation_settings installation ON installation.singleton = settings.singleton
		WHERE settings.singleton = TRUE
		  AND (($1 = 'setup' AND installation.setup_state = 'configuring')
		       OR ($1 = 'login' AND installation.setup_state = 'ready'))
	`
	if attempt.Purpose == installation.OIDCLoginPurposeSettingsVerification {
		configurationQuery = `
			SELECT provider_name, issuer_url, client_id, client_secret, additional_scopes,
			       username_claim, display_name_claim, avatar_claim, groups_claim, access_urls,
			       button_text, redirect_uri, updated_at
			FROM oidc_pending_settings WHERE singleton = TRUE
		`
	}
	var configurationRow pgx.Row
	if attempt.Purpose == installation.OIDCLoginPurposeSettingsVerification {
		configurationRow = transaction.QueryRow(ctx, configurationQuery)
	} else {
		configurationRow = transaction.QueryRow(ctx, configurationQuery, attempt.Purpose)
	}
	err = configurationRow.Scan(
		&configuration.Name,
		&configuration.IssuerURL,
		&configuration.ClientID,
		&configuration.ClientSecret,
		&configuration.AdditionalScopes,
		&configuration.UsernameClaim,
		&configuration.DisplayNameClaim,
		&configuration.AvatarClaim,
		&configuration.GroupsClaim,
		&configuration.AccessURLs,
		&configuration.ButtonText,
		&configuration.RedirectURI,
		&configuration.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return installation.OIDCConfiguration{}, installation.OIDCLoginAttempt{}, identity.ErrLoginAttemptInvalid
	}
	if err != nil {
		return installation.OIDCConfiguration{}, installation.OIDCLoginAttempt{}, fmt.Errorf("load OIDC callback configuration: %w", err)
	}

	if err := transaction.Commit(ctx); err != nil {
		return installation.OIDCConfiguration{}, installation.OIDCLoginAttempt{}, fmt.Errorf("commit OIDC callback transaction: %w", err)
	}
	return configuration, attempt, nil
}

func (store *OIDCSetupStore) PendingAdministratorAllowed(ctx context.Context, groups []string) (bool, error) {
	var allowed bool
	err := store.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM oidc_pending_settings
			WHERE singleton = TRUE AND admin_groups && $1::text[]
		)
	`, nonNilStrings(groups)).Scan(&allowed)
	if err != nil {
		return false, fmt.Errorf("check pending administrator access: %w", err)
	}
	return allowed, nil
}

func (store *OIDCSetupStore) CoreAdministratorAllowed(ctx context.Context, groups []string) (bool, error) {
	var allowed bool
	err := store.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM services
			JOIN service_roles roles
			  ON roles.service_id = services.id
			 AND roles.role_key = 'admin'
			 AND roles.active = TRUE
			JOIN service_role_groups mappings
			  ON mappings.service_id = roles.service_id
			 AND mappings.role_key = roles.role_key
			WHERE services.service_type = 'core'
			  AND mappings.oidc_group = ANY($1::text[])
		)
	`, nonNilStrings(groups)).Scan(&allowed)
	if err != nil {
		return false, fmt.Errorf("check Kaeru Core administrator access: %w", err)
	}
	return allowed, nil
}

func (store *OIDCSetupStore) BootstrapAdministrator(
	ctx context.Context,
	user identity.OIDCIdentity,
	session identity.BrowserSession,
	now time.Time,
) error {
	transaction, err := store.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return fmt.Errorf("begin administrator bootstrap transaction: %w", err)
	}
	defer func() { _ = transaction.Rollback(ctx) }()

	result, err := transaction.Exec(ctx, `
		UPDATE installation_settings
		SET setup_state = 'ready', updated_at = $1
		WHERE singleton = TRUE AND setup_state = 'configuring'
	`, now)
	if err != nil {
		return fmt.Errorf("complete installation setup: %w", err)
	}
	if result.RowsAffected() != 1 {
		return installation.ErrAlreadyInitialized
	}
	username, err := allocateUniqueUsername(ctx, transaction, user.Username)
	if err != nil {
		return fmt.Errorf("allocate initial administrator username: %w", err)
	}

	_, err = transaction.Exec(ctx, `
		INSERT INTO users (
			id, oidc_issuer, oidc_subject, username, display_name, email, email_verified,
			oidc_avatar_url, created_at, updated_at, last_login_at, last_seen_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $9, $9, $9)
	`,
		user.ID,
		user.Issuer,
		user.Subject,
		username,
		user.DisplayName,
		user.Email,
		user.EmailVerified,
		user.AvatarURL,
		now,
	)
	if err != nil {
		return fmt.Errorf("create initial administrator: %w", err)
	}
	for _, group := range user.Groups {
		if _, err := transaction.Exec(ctx, `
			INSERT INTO user_oidc_groups (user_id, group_name)
			VALUES ($1, $2)
		`, user.ID, group); err != nil {
			return fmt.Errorf("save initial administrator groups: %w", err)
		}
	}
	_, err = transaction.Exec(ctx, `
		INSERT INTO user_sessions (
			id, user_id, token_hash, created_at, last_seen_at, expires_at,
			user_agent, ip_address
		) VALUES ($1, $2, $3, $4, $4, $5, $6, $7)
	`,
		session.ID,
		user.ID,
		session.TokenHash[:],
		session.CreatedAt,
		session.ExpiresAt,
		nullableText(session.UserAgent),
		nullableText(session.IPAddress),
	)
	if err != nil {
		return fmt.Errorf("create administrator session: %w", err)
	}

	if err := transaction.Commit(ctx); err != nil {
		return fmt.Errorf("commit administrator bootstrap: %w", err)
	}
	return nil
}

func (store *OIDCSetupStore) CompleteUserLogin(
	ctx context.Context,
	user identity.OIDCIdentity,
	session identity.BrowserSession,
	now time.Time,
) error {
	transaction, err := store.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return fmt.Errorf("begin user login transaction: %w", err)
	}
	defer func() { _ = transaction.Rollback(ctx) }()
	if err := lockUsernameAllocation(ctx, transaction); err != nil {
		return err
	}

	userID := ""
	var disabledAt *time.Time
	err = transaction.QueryRow(ctx, `
		SELECT id, disabled_at FROM users
		WHERE oidc_issuer = $1 AND oidc_subject = $2
	`, user.Issuer, user.Subject).Scan(&userID, &disabledAt)
	if errors.Is(err, pgx.ErrNoRows) {
		username, allocationErr := allocateUniqueUsernameLocked(ctx, transaction, user.Username)
		if allocationErr != nil {
			return fmt.Errorf("allocate OIDC username: %w", allocationErr)
		}
		userID = user.ID
		_, err = transaction.Exec(ctx, `
			INSERT INTO users (
				id, oidc_issuer, oidc_subject, username, display_name, email, email_verified,
				oidc_avatar_url, created_at, updated_at, last_login_at, last_seen_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $9, $9, $9)
		`, userID, user.Issuer, user.Subject, username, user.DisplayName, user.Email, user.EmailVerified, user.AvatarURL, now)
		if err != nil {
			return fmt.Errorf("create OIDC user: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("find OIDC user: %w", err)
	} else {
		if disabledAt != nil {
			return identity.ErrUserDisabled
		}
		_, err = transaction.Exec(ctx, `
			UPDATE users
			SET oidc_avatar_url = $2, updated_at = $3, last_login_at = $3, last_seen_at = $3
			WHERE id = $1
		`, userID, user.AvatarURL, now)
		if err != nil {
			return fmt.Errorf("update OIDC user login: %w", err)
		}
	}

	if _, err := transaction.Exec(ctx, `DELETE FROM user_oidc_groups WHERE user_id = $1`, userID); err != nil {
		return fmt.Errorf("replace OIDC user groups: %w", err)
	}
	for _, group := range user.Groups {
		if _, err := transaction.Exec(ctx, `
			INSERT INTO user_oidc_groups (user_id, group_name) VALUES ($1, $2)
		`, userID, group); err != nil {
			return fmt.Errorf("save OIDC user group: %w", err)
		}
	}
	_, err = transaction.Exec(ctx, `
		INSERT INTO user_sessions (
			id, user_id, token_hash, created_at, last_seen_at, expires_at, user_agent, ip_address
		) VALUES ($1, $2, $3, $4, $4, $5, $6, $7)
	`, session.ID, userID, session.TokenHash[:], session.CreatedAt, session.ExpiresAt, nullableText(session.UserAgent), nullableText(session.IPAddress))
	if err != nil {
		return fmt.Errorf("create user session: %w", err)
	}
	if err := transaction.Commit(ctx); err != nil {
		return fmt.Errorf("commit user login: %w", err)
	}
	return nil
}

func (store *OIDCSetupStore) CompleteOIDCSettingsVerification(
	ctx context.Context,
	user identity.OIDCIdentity,
	session identity.BrowserSession,
	now time.Time,
) error {
	transaction, err := store.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return fmt.Errorf("begin OIDC settings promotion transaction: %w", err)
	}
	defer func() { _ = transaction.Rollback(ctx) }()
	if err := lockUsernameAllocation(ctx, transaction); err != nil {
		return err
	}

	currentAuthentication := installation.OIDCConfiguration{}
	pendingAuthentication := installation.OIDCConfiguration{}
	err = transaction.QueryRow(ctx, `
		SELECT
			active.issuer_url, active.client_id, active.client_secret,
			active.additional_scopes, active.username_claim, active.groups_claim,
			active.access_urls,
			pending.issuer_url, pending.client_id, pending.client_secret,
			pending.additional_scopes, pending.username_claim, pending.groups_claim,
			pending.access_urls
		FROM oidc_settings active
		JOIN oidc_pending_settings pending ON pending.singleton = active.singleton
		WHERE active.singleton = TRUE
	`).Scan(
		&currentAuthentication.IssuerURL,
		&currentAuthentication.ClientID,
		&currentAuthentication.ClientSecret,
		&currentAuthentication.AdditionalScopes,
		&currentAuthentication.UsernameClaim,
		&currentAuthentication.GroupsClaim,
		&currentAuthentication.AccessURLs,
		&pendingAuthentication.IssuerURL,
		&pendingAuthentication.ClientID,
		&pendingAuthentication.ClientSecret,
		&pendingAuthentication.AdditionalScopes,
		&pendingAuthentication.UsernameClaim,
		&pendingAuthentication.GroupsClaim,
		&pendingAuthentication.AccessURLs,
	)
	if err != nil {
		return fmt.Errorf("compare verified OIDC authentication settings: %w", err)
	}
	revokeExistingSessions := oidcAuthenticationRequiresSessionRevocation(
		currentAuthentication,
		pendingAuthentication,
	)

	userID := ""
	var disabledAt *time.Time
	err = transaction.QueryRow(ctx, `
		SELECT id, disabled_at FROM users WHERE oidc_issuer = $1 AND oidc_subject = $2
	`, user.Issuer, user.Subject).Scan(&userID, &disabledAt)
	if errors.Is(err, pgx.ErrNoRows) {
		username, allocationErr := allocateUniqueUsernameLocked(ctx, transaction, user.Username)
		if allocationErr != nil {
			return fmt.Errorf("allocate verified OIDC username: %w", allocationErr)
		}
		userID = user.ID
		_, err = transaction.Exec(ctx, `
			INSERT INTO users (
				id, oidc_issuer, oidc_subject, username, display_name, email, email_verified,
				oidc_avatar_url, created_at, updated_at, last_login_at, last_seen_at
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$9,$9,$9)
		`, userID, user.Issuer, user.Subject, username, user.DisplayName, user.Email,
			user.EmailVerified, user.AvatarURL, now)
	} else if err == nil && disabledAt != nil {
		return identity.ErrUserDisabled
	} else if err == nil {
		_, err = transaction.Exec(ctx, `
			UPDATE users SET oidc_avatar_url=$2, updated_at=$3, last_login_at=$3, last_seen_at=$3
			WHERE id=$1
		`, userID, user.AvatarURL, now)
	}
	if err != nil {
		return fmt.Errorf("save verified OIDC user: %w", err)
	}

	result, err := transaction.Exec(ctx, `
		UPDATE oidc_settings active SET
			provider_name=pending.provider_name, issuer_url=pending.issuer_url,
			client_id=pending.client_id, client_secret=pending.client_secret,
			additional_scopes=pending.additional_scopes, username_claim=pending.username_claim,
			display_name_claim=pending.display_name_claim, avatar_claim=pending.avatar_claim,
			groups_claim=pending.groups_claim, access_urls=pending.access_urls,
			button_text=pending.button_text, button_image=pending.button_image,
			button_image_content_type=pending.button_image_content_type,
			redirect_uri=pending.redirect_uri, updated_at=$1
		FROM oidc_pending_settings pending
		WHERE active.singleton=TRUE AND pending.singleton=TRUE
	`, now)
	if err != nil {
		return fmt.Errorf("promote verified OIDC settings: %w", err)
	}
	if result.RowsAffected() != 1 {
		return errors.New("pending OIDC settings are unavailable")
	}
	if _, err := transaction.Exec(ctx, `
		DELETE FROM service_role_groups USING services
		WHERE service_role_groups.service_id=services.id
		  AND services.service_type='core' AND service_role_groups.role_key='admin'
	`); err != nil {
		return fmt.Errorf("replace verified administrator mappings: %w", err)
	}
	if _, err := transaction.Exec(ctx, `
		INSERT INTO service_role_groups (service_id, role_key, oidc_group)
		SELECT services.id, 'admin', unnest(pending.admin_groups)
		FROM services, oidc_pending_settings pending
		WHERE services.service_type='core' AND pending.singleton=TRUE
	`); err != nil {
		return fmt.Errorf("save verified administrator mappings: %w", err)
	}
	if revokeExistingSessions {
		if _, err := transaction.Exec(ctx, `DELETE FROM user_sessions`); err != nil {
			return fmt.Errorf("revoke existing sessions: %w", err)
		}
	}
	if _, err := transaction.Exec(ctx, `DELETE FROM user_oidc_groups WHERE user_id=$1`, userID); err != nil {
		return fmt.Errorf("replace verified user groups: %w", err)
	}
	for _, group := range user.Groups {
		if _, err := transaction.Exec(ctx, `INSERT INTO user_oidc_groups (user_id, group_name) VALUES ($1,$2)`, userID, group); err != nil {
			return fmt.Errorf("save verified user group: %w", err)
		}
	}
	if _, err := transaction.Exec(ctx, `
		INSERT INTO user_sessions (
			id,user_id,token_hash,created_at,last_seen_at,expires_at,user_agent,ip_address
		) VALUES ($1,$2,$3,$4,$4,$5,$6,$7)
	`, session.ID, userID, session.TokenHash[:], session.CreatedAt, session.ExpiresAt,
		nullableText(session.UserAgent), nullableText(session.IPAddress)); err != nil {
		return fmt.Errorf("create verified session: %w", err)
	}
	if _, err := transaction.Exec(ctx, `DELETE FROM oidc_pending_settings WHERE singleton=TRUE`); err != nil {
		return fmt.Errorf("clear pending OIDC settings: %w", err)
	}
	if err := transaction.Commit(ctx); err != nil {
		return fmt.Errorf("commit verified OIDC settings: %w", err)
	}
	return nil
}

func oidcAuthenticationRequiresSessionRevocation(
	current installation.OIDCConfiguration,
	pending installation.OIDCConfiguration,
) bool {
	return current.IssuerURL != pending.IssuerURL ||
		current.ClientID != pending.ClientID ||
		current.ClientSecret != pending.ClientSecret ||
		!slices.Equal(current.AdditionalScopes, pending.AdditionalScopes) ||
		current.UsernameClaim != pending.UsernameClaim ||
		current.GroupsClaim != pending.GroupsClaim ||
		!slices.Equal(current.AccessURLs, pending.AccessURLs)
}

func (store *OIDCSetupStore) LoadOIDCConfiguration(ctx context.Context) (installation.OIDCConfiguration, error) {
	configuration := installation.OIDCConfiguration{}
	err := store.pool.QueryRow(ctx, `
		SELECT settings.provider_name, settings.issuer_url, settings.client_id, settings.client_secret,
		       settings.additional_scopes, settings.username_claim, settings.display_name_claim, settings.avatar_claim,
		       settings.groups_claim, settings.access_urls, settings.button_text,
		       settings.redirect_uri, settings.updated_at
		FROM oidc_settings settings
		JOIN installation_settings installation ON installation.singleton = settings.singleton
		WHERE settings.singleton = TRUE AND installation.setup_state = 'ready'
	`).Scan(
		&configuration.Name,
		&configuration.IssuerURL, &configuration.ClientID, &configuration.ClientSecret,
		&configuration.AdditionalScopes, &configuration.UsernameClaim, &configuration.DisplayNameClaim, &configuration.AvatarClaim,
		&configuration.GroupsClaim, &configuration.AccessURLs, &configuration.ButtonText,
		&configuration.RedirectURI, &configuration.UpdatedAt,
	)
	if err != nil {
		return installation.OIDCConfiguration{}, fmt.Errorf("load OIDC login configuration: %w", err)
	}
	return configuration, nil
}

func (store *OIDCSetupStore) LoadOIDCSettings(ctx context.Context) (installation.OIDCSettings, error) {
	settings := installation.OIDCSettings{}
	err := store.pool.QueryRow(ctx, `
		SELECT settings.provider_name,
		       core.public_url,
		       settings.access_urls,
		       settings.issuer_url,
		       settings.client_id,
		       settings.client_secret <> '',
		       settings.additional_scopes,
		       settings.username_claim,
		       settings.display_name_claim,
		       settings.avatar_claim,
		       settings.groups_claim,
		       COALESCE((
		           SELECT array_agg(groups.oidc_group ORDER BY groups.oidc_group)
		           FROM service_role_groups groups
		           WHERE groups.service_id = core.id AND groups.role_key = 'admin'
		       ), '{}'),
		       settings.button_text,
		       settings.button_image IS NOT NULL,
		       settings.redirect_uri
		FROM oidc_settings settings
		JOIN installation_settings installation ON installation.singleton = settings.singleton
		JOIN services core ON core.service_type = 'core'
		WHERE settings.singleton = TRUE AND installation.setup_state = 'ready'
	`).Scan(
		&settings.Name,
		&settings.PublicURL,
		&settings.AccessURLs,
		&settings.IssuerURL,
		&settings.ClientID,
		&settings.ClientSecretSet,
		&settings.AdditionalScopes,
		&settings.UsernameClaim,
		&settings.DisplayNameClaim,
		&settings.AvatarClaim,
		&settings.GroupsClaim,
		&settings.AdminGroups,
		&settings.ButtonText,
		&settings.ButtonImageConfigured,
		&settings.CallbackURL,
	)
	if err != nil {
		return installation.OIDCSettings{}, fmt.Errorf("load OIDC settings: %w", err)
	}
	return settings, nil
}

func (store *OIDCSetupStore) LoadOIDCButtonImage(ctx context.Context) (installation.OIDCButtonImage, error) {
	image := installation.OIDCButtonImage{}
	err := store.pool.QueryRow(ctx, `
		SELECT button_image, button_image_content_type
		FROM oidc_settings
		WHERE singleton = TRUE AND button_image IS NOT NULL
	`).Scan(&image.Content, &image.ContentType)
	if err != nil {
		return installation.OIDCButtonImage{}, fmt.Errorf("load OIDC button image: %w", err)
	}
	return image, nil
}

func (store *OIDCSetupStore) LoadOIDCBranding(ctx context.Context) (installation.OIDCBranding, error) {
	branding := installation.OIDCBranding{}
	err := store.pool.QueryRow(ctx, `
		SELECT provider_name, button_text, button_image IS NOT NULL
		FROM oidc_settings settings
		JOIN installation_settings installation ON installation.singleton = settings.singleton
		WHERE settings.singleton = TRUE AND installation.setup_state = 'ready'
	`).Scan(&branding.Name, &branding.ButtonText, &branding.ButtonImageConfigured)
	if err != nil {
		return installation.OIDCBranding{}, fmt.Errorf("load OIDC branding: %w", err)
	}
	return branding, nil
}

func (store *OIDCSetupStore) UpdateOIDCConfiguration(ctx context.Context, configuration installation.OIDCConfiguration) error {
	transaction, err := store.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin OIDC settings transaction: %w", err)
	}
	defer func() { _ = transaction.Rollback(ctx) }()

	result, err := transaction.Exec(ctx, `
		UPDATE oidc_settings
		SET provider_name = $1,
		    issuer_url = $2,
		    client_id = $3,
		    client_secret = $4,
		    additional_scopes = $5,
		    username_claim = $6,
		    display_name_claim = $7,
		    avatar_claim = $8,
		    groups_claim = $9,
		    access_urls = $10,
		    button_text = $11,
		    button_image = COALESCE($12::bytea, button_image),
		    button_image_content_type = CASE WHEN $12::bytea IS NULL THEN button_image_content_type ELSE $13 END,
		    redirect_uri = $14,
		    updated_at = $15
		WHERE singleton = TRUE
	`,
		configuration.Name,
		configuration.IssuerURL,
		configuration.ClientID,
		configuration.ClientSecret,
		nonNilStrings(configuration.AdditionalScopes),
		configuration.UsernameClaim,
		configuration.DisplayNameClaim,
		configuration.AvatarClaim,
		configuration.GroupsClaim,
		nonNilStrings(configuration.AccessURLs),
		configuration.ButtonText,
		nullableBytes(configuration.ButtonImage),
		nullableText(configuration.ButtonImageContentType),
		configuration.RedirectURI,
		configuration.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("update OIDC settings: %w", err)
	}
	if result.RowsAffected() != 1 {
		return errors.New("OIDC settings are unavailable")
	}

	if _, err := transaction.Exec(ctx, `
		DELETE FROM service_role_groups
		USING services
		WHERE service_role_groups.service_id = services.id
		  AND services.service_type = 'core'
		  AND service_role_groups.role_key = 'admin'
	`); err != nil {
		return fmt.Errorf("replace Kaeru Core administrator mappings: %w", err)
	}
	for _, group := range configuration.AdminGroups {
		result, err = transaction.Exec(ctx, `
			INSERT INTO service_role_groups (service_id, role_key, oidc_group)
			SELECT services.id, 'admin', $1
			FROM services
			JOIN service_roles ON service_roles.service_id = services.id AND service_roles.role_key = 'admin'
			WHERE services.service_type = 'core'
		`, group)
		if err != nil {
			return fmt.Errorf("save Kaeru Core administrator mapping: %w", err)
		}
		if result.RowsAffected() != 1 {
			return errors.New("Kaeru Core administrator role is unavailable")
		}
	}
	if err := transaction.Commit(ctx); err != nil {
		return fmt.Errorf("commit OIDC settings: %w", err)
	}
	return nil
}

func (store *OIDCSetupStore) SaveOIDCLoginAttempt(ctx context.Context, attempt installation.OIDCLoginAttempt) error {
	_, err := store.pool.Exec(ctx, `
		WITH expired_attempts AS (
			DELETE FROM oidc_login_attempts WHERE expires_at <= $6
		)
		INSERT INTO oidc_login_attempts (
			state_hash, code_verifier, nonce, redirect_uri, expires_at, created_at, purpose
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, attempt.StateHash[:], attempt.CodeVerifier, attempt.Nonce, attempt.RedirectURI, attempt.ExpiresAt, attempt.CreatedAt, attempt.Purpose)
	if err != nil {
		return fmt.Errorf("save OIDC login attempt: %w", err)
	}
	return nil
}

func (store *OIDCSetupStore) SavePendingOIDCConfiguration(
	ctx context.Context,
	configuration installation.OIDCConfiguration,
	attempt installation.OIDCLoginAttempt,
) error {
	transaction, err := store.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin pending OIDC settings transaction: %w", err)
	}
	defer func() { _ = transaction.Rollback(ctx) }()

	_, err = transaction.Exec(ctx, `
		INSERT INTO oidc_pending_settings (
			singleton, provider_name, issuer_url, client_id, client_secret, additional_scopes,
			username_claim, display_name_claim, avatar_claim, groups_claim, admin_groups,
			access_urls, button_text, button_image, button_image_content_type, redirect_uri, updated_at
		) VALUES (
			TRUE, $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12,
			COALESCE($13::bytea, (SELECT button_image FROM oidc_settings WHERE singleton = TRUE)),
			CASE WHEN $13::bytea IS NULL
				THEN (SELECT button_image_content_type FROM oidc_settings WHERE singleton = TRUE)
				ELSE $14 END,
			$15, $16
		)
		ON CONFLICT (singleton) DO UPDATE SET
			provider_name = EXCLUDED.provider_name, issuer_url = EXCLUDED.issuer_url,
			client_id = EXCLUDED.client_id, client_secret = EXCLUDED.client_secret,
			additional_scopes = EXCLUDED.additional_scopes, username_claim = EXCLUDED.username_claim,
			display_name_claim = EXCLUDED.display_name_claim, avatar_claim = EXCLUDED.avatar_claim,
			groups_claim = EXCLUDED.groups_claim, admin_groups = EXCLUDED.admin_groups,
			access_urls = EXCLUDED.access_urls, button_text = EXCLUDED.button_text,
			button_image = CASE WHEN $13::bytea IS NULL THEN oidc_pending_settings.button_image ELSE EXCLUDED.button_image END,
			button_image_content_type = CASE WHEN $13::bytea IS NULL THEN oidc_pending_settings.button_image_content_type ELSE EXCLUDED.button_image_content_type END,
			redirect_uri = EXCLUDED.redirect_uri, updated_at = EXCLUDED.updated_at
	`, configuration.Name, configuration.IssuerURL, configuration.ClientID, configuration.ClientSecret,
		nonNilStrings(configuration.AdditionalScopes), configuration.UsernameClaim, configuration.DisplayNameClaim,
		configuration.AvatarClaim, configuration.GroupsClaim, nonNilStrings(configuration.AdminGroups),
		nonNilStrings(configuration.AccessURLs), configuration.ButtonText, nullableBytes(configuration.ButtonImage),
		nullableText(configuration.ButtonImageContentType), configuration.RedirectURI, configuration.UpdatedAt)
	if err != nil {
		return fmt.Errorf("save pending OIDC settings: %w", err)
	}
	if _, err := transaction.Exec(ctx, `
		WITH expired_attempts AS (DELETE FROM oidc_login_attempts WHERE expires_at <= $6)
		INSERT INTO oidc_login_attempts (
			state_hash, code_verifier, nonce, redirect_uri, expires_at, created_at, purpose
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, attempt.StateHash[:], attempt.CodeVerifier, attempt.Nonce, attempt.RedirectURI,
		attempt.ExpiresAt, attempt.CreatedAt, attempt.Purpose); err != nil {
		return fmt.Errorf("save OIDC settings verification attempt: %w", err)
	}
	if err := transaction.Commit(ctx); err != nil {
		return fmt.Errorf("commit pending OIDC settings: %w", err)
	}
	return nil
}

func NewOIDCSetupStore(pool *pgxpool.Pool) *OIDCSetupStore {
	return &OIDCSetupStore{pool: pool}
}

func (store *OIDCSetupStore) SaveOIDCSetup(
	ctx context.Context,
	configuration installation.OIDCConfiguration,
	attempt installation.OIDCLoginAttempt,
) error {
	transaction, err := store.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin OIDC setup transaction: %w", err)
	}
	defer func() { _ = transaction.Rollback(ctx) }()

	result, err := transaction.Exec(ctx, `
		UPDATE installation_settings
		SET setup_state = 'configuring', updated_at = $1
		WHERE singleton = TRUE
		  AND setup_state IN ('required', 'configuring')
	`, configuration.UpdatedAt)
	if err != nil {
		return fmt.Errorf("update installation state for OIDC setup: %w", err)
	}
	if result.RowsAffected() != 1 {
		return installation.ErrAlreadyInitialized
	}

	_, err = transaction.Exec(ctx, `
		INSERT INTO oidc_settings (
			singleton, provider_name, issuer_url, client_id, client_secret, additional_scopes,
			username_claim, display_name_claim, avatar_claim, groups_claim, access_urls, button_text,
			button_image, button_image_content_type, redirect_uri, updated_at
		) VALUES (TRUE, $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		ON CONFLICT (singleton) DO UPDATE SET
			provider_name = EXCLUDED.provider_name,
			issuer_url = EXCLUDED.issuer_url,
			client_id = EXCLUDED.client_id,
			client_secret = EXCLUDED.client_secret,
			additional_scopes = EXCLUDED.additional_scopes,
			username_claim = EXCLUDED.username_claim,
			display_name_claim = EXCLUDED.display_name_claim,
			avatar_claim = EXCLUDED.avatar_claim,
			groups_claim = EXCLUDED.groups_claim,
			access_urls = EXCLUDED.access_urls,
			button_text = EXCLUDED.button_text,
			button_image = COALESCE(EXCLUDED.button_image, oidc_settings.button_image),
			button_image_content_type = CASE
				WHEN EXCLUDED.button_image IS NULL THEN oidc_settings.button_image_content_type
				ELSE EXCLUDED.button_image_content_type
			END,
			redirect_uri = EXCLUDED.redirect_uri,
			updated_at = EXCLUDED.updated_at
	`,
		configuration.Name,
		configuration.IssuerURL,
		configuration.ClientID,
		configuration.ClientSecret,
		nonNilStrings(configuration.AdditionalScopes),
		configuration.UsernameClaim,
		configuration.DisplayNameClaim,
		configuration.AvatarClaim,
		configuration.GroupsClaim,
		nonNilStrings(configuration.AccessURLs),
		configuration.ButtonText,
		nullableBytes(configuration.ButtonImage),
		nullableText(configuration.ButtonImageContentType),
		configuration.RedirectURI,
		configuration.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("save OIDC configuration: %w", err)
	}

	result, err = transaction.Exec(ctx, `
		UPDATE services
		SET public_url = $1
		WHERE service_type = 'core'
	`, configuration.PublicURL)
	if err != nil {
		return fmt.Errorf("save Kaeru Core public URL: %w", err)
	}
	if result.RowsAffected() != 1 {
		return errors.New("Kaeru Core service is unavailable")
	}

	result, err = transaction.Exec(ctx, `
		DELETE FROM service_role_groups
		USING services
		WHERE service_role_groups.service_id = services.id
		  AND services.service_type = 'core'
		  AND service_role_groups.role_key = 'admin'
	`)
	if err != nil {
		return fmt.Errorf("replace Kaeru Core administrator mapping: %w", err)
	}
	for _, group := range configuration.AdminGroups {
		result, err = transaction.Exec(ctx, `
			INSERT INTO service_role_groups (service_id, role_key, oidc_group)
			SELECT services.id, 'admin', $1
			FROM services
			JOIN service_roles
			  ON service_roles.service_id = services.id
			 AND service_roles.role_key = 'admin'
			WHERE services.service_type = 'core'
		`, group)
		if err != nil {
			return fmt.Errorf("save Kaeru Core administrator mapping: %w", err)
		}
		if result.RowsAffected() != 1 {
			return errors.New("Kaeru Core administrator role is unavailable")
		}
	}

	if _, err := transaction.Exec(ctx, `DELETE FROM oidc_login_attempts`); err != nil {
		return fmt.Errorf("clear previous OIDC login attempts: %w", err)
	}
	_, err = transaction.Exec(ctx, `
		INSERT INTO oidc_login_attempts (
			state_hash, code_verifier, nonce, redirect_uri, expires_at, created_at, purpose
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, attempt.StateHash[:], attempt.CodeVerifier, attempt.Nonce, attempt.RedirectURI, attempt.ExpiresAt, attempt.CreatedAt, attempt.Purpose)
	if err != nil {
		return fmt.Errorf("save OIDC login attempt: %w", err)
	}

	if err := transaction.Commit(ctx); err != nil {
		return fmt.Errorf("commit OIDC setup: %w", err)
	}
	return nil
}

func nullableBytes(value []byte) any {
	if len(value) == 0 {
		return nil
	}
	return value
}

func nullableText(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func nonNilStrings(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}
