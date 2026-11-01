package repositories

import (
	"errors"
	"sort"
	"strings"

	"github.com/beego/beego/v2/client/orm"

	"github.com/lingster/go-rest-template/backend/models"
)

// ---- shared helpers (DRY) ----

func one(qs orm.QuerySeter, dest interface{}, cols ...string) error {
	err := qs.One(dest, cols...)
	if errors.Is(err, orm.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

func insert(m interface{}) error {
	_, err := orm.NewOrm().Insert(m)
	if err != nil {
		msg := strings.ToLower(err.Error())
		if strings.Contains(msg, "unique") || strings.Contains(msg, "duplicate") {
			return ErrDuplicate
		}
	}
	return err
}

func update(m interface{}, cols ...string) error {
	_, err := orm.NewOrm().Update(m, cols...)
	return err
}

func deleteWhere(model interface{}, field string, value interface{}) error {
	_, err := orm.NewOrm().QueryTable(model).Filter(field, value).Delete()
	return err
}

// markFlag atomically flips a boolean column from false to true.
func markFlag(model interface{}, id, column string) (bool, error) {
	n, err := orm.NewOrm().QueryTable(model).Filter("id", id).Filter(column, false).
		Update(orm.Params{column: true})
	return n > 0, err
}

// ---- users ----

type ormUserRepo struct{}

func (*ormUserRepo) Create(u *models.User) error { return insert(u) }

func (*ormUserRepo) GetByID(id string) (*models.User, error) {
	u := &models.User{}
	return u, one(orm.NewOrm().QueryTable(u).Filter("id", id), u)
}

func (*ormUserRepo) GetByEmail(email string) (*models.User, error) {
	u := &models.User{}
	return u, one(orm.NewOrm().QueryTable(u).Filter("email", strings.ToLower(email)), u)
}

func (*ormUserRepo) Update(u *models.User, columns ...string) error { return update(u, columns...) }

func (*ormUserRepo) Delete(id string) error { return deleteWhere(new(models.User), "id", id) }

func (*ormUserRepo) AdvanceTOTPStep(id string, step int64) (bool, error) {
	n, err := orm.NewOrm().QueryTable(new(models.User)).Filter("id", id).Filter("totp_last_step__lt", step).
		Update(orm.Params{"totp_last_step": step})
	return n > 0, err
}

func (*ormUserRepo) List(q string, offset, limit int) ([]*models.User, int64, error) {
	qs := orm.NewOrm().QueryTable(new(models.User))
	if q = strings.TrimSpace(q); q != "" {
		cond := orm.NewCondition().Or("email__icontains", q).Or("name__icontains", q)
		qs = qs.SetCond(cond)
	}
	total, err := qs.Count()
	if err != nil {
		return nil, 0, err
	}
	var users []*models.User
	_, err = qs.OrderBy("-created_at", "id").Limit(limit, offset).All(&users)
	return users, total, err
}

func (*ormUserRepo) Stats() (UserStats, error) {
	q := func() orm.QuerySeter { return orm.NewOrm().QueryTable(new(models.User)) }
	var s UserStats
	var err error
	if s.Total, err = q().Count(); err != nil {
		return s, err
	}
	if s.Active, err = q().Filter("is_active", true).Count(); err != nil {
		return s, err
	}
	if s.Verified, err = q().Filter("email_verified", true).Count(); err != nil {
		return s, err
	}
	s.TwoFA, err = q().Filter("totp_enabled", true).Count()
	return s, err
}

// ---- refresh tokens ----

type ormRefreshRepo struct{}

func (*ormRefreshRepo) Create(t *models.RefreshToken) error { return insert(t) }

func (*ormRefreshRepo) GetByHash(hash string) (*models.RefreshToken, error) {
	t := &models.RefreshToken{}
	return t, one(orm.NewOrm().QueryTable(t).Filter("token_hash", hash), t)
}

func (*ormRefreshRepo) Revoke(id string) (bool, error) {
	return markFlag(new(models.RefreshToken), id, "revoked")
}

func (*ormRefreshRepo) RevokeFamily(familyID string) error {
	_, err := orm.NewOrm().QueryTable(new(models.RefreshToken)).Filter("family_id", familyID).
		Update(orm.Params{"revoked": true})
	return err
}

func (*ormRefreshRepo) RevokeAllForUser(userID string) error {
	_, err := orm.NewOrm().QueryTable(new(models.RefreshToken)).Filter("user_id", userID).
		Filter("revoked", false).Update(orm.Params{"revoked": true})
	return err
}

func (*ormRefreshRepo) DeleteByUser(userID string) error {
	return deleteWhere(new(models.RefreshToken), "user_id", userID)
}

// ---- one-time tokens ----

type ormOneTimeRepo struct{}

func (*ormOneTimeRepo) Create(t *models.OneTimeToken) error { return insert(t) }

func (*ormOneTimeRepo) GetByHash(hash string) (*models.OneTimeToken, error) {
	t := &models.OneTimeToken{}
	return t, one(orm.NewOrm().QueryTable(t).Filter("token_hash", hash), t)
}

func (*ormOneTimeRepo) MarkUsed(id string) (bool, error) {
	return markFlag(new(models.OneTimeToken), id, "used")
}

func (*ormOneTimeRepo) InvalidateOutstanding(userID, purpose string) error {
	_, err := orm.NewOrm().QueryTable(new(models.OneTimeToken)).Filter("user_id", userID).
		Filter("purpose", purpose).Filter("used", false).Update(orm.Params{"used": true})
	return err
}

func (*ormOneTimeRepo) DeleteByUser(userID string) error {
	return deleteWhere(new(models.OneTimeToken), "user_id", userID)
}

// ---- recovery codes ----

type ormRecoveryRepo struct{}

func (*ormRecoveryRepo) ReplaceForUser(userID string, codes []*models.RecoveryCode) error {
	if err := deleteWhere(new(models.RecoveryCode), "user_id", userID); err != nil {
		return err
	}
	if len(codes) == 0 {
		return nil
	}
	_, err := orm.NewOrm().InsertMulti(len(codes), codes)
	return err
}

func (*ormRecoveryRepo) FindUnused(userID, hash string) (*models.RecoveryCode, error) {
	c := &models.RecoveryCode{}
	return c, one(orm.NewOrm().QueryTable(c).Filter("user_id", userID).Filter("code_hash", hash).Filter("used", false), c)
}

func (*ormRecoveryRepo) MarkUsed(id string) (bool, error) {
	return markFlag(new(models.RecoveryCode), id, "used")
}

func (*ormRecoveryRepo) CountUnused(userID string) (int64, error) {
	return orm.NewOrm().QueryTable(new(models.RecoveryCode)).Filter("user_id", userID).Filter("used", false).Count()
}

func (*ormRecoveryRepo) DeleteByUser(userID string) error {
	return deleteWhere(new(models.RecoveryCode), "user_id", userID)
}

// ---- passkeys ----

type ormPasskeyRepo struct{}

func (*ormPasskeyRepo) Create(c *models.WebAuthnCredential) error { return insert(c) }

func (*ormPasskeyRepo) ListByUser(userID string) ([]*models.WebAuthnCredential, error) {
	var out []*models.WebAuthnCredential
	_, err := orm.NewOrm().QueryTable(new(models.WebAuthnCredential)).Filter("user_id", userID).
		OrderBy("created_at").All(&out)
	return out, err
}

func (*ormPasskeyRepo) GetByCredentialID(credentialID string) (*models.WebAuthnCredential, error) {
	c := &models.WebAuthnCredential{}
	return c, one(orm.NewOrm().QueryTable(c).Filter("credential_id", credentialID), c)
}

func (*ormPasskeyRepo) GetForUser(id, userID string) (*models.WebAuthnCredential, error) {
	c := &models.WebAuthnCredential{}
	return c, one(orm.NewOrm().QueryTable(c).Filter("id", id).Filter("user_id", userID), c)
}

func (*ormPasskeyRepo) Update(c *models.WebAuthnCredential, columns ...string) error {
	return update(c, columns...)
}

func (*ormPasskeyRepo) Delete(id string) error {
	return deleteWhere(new(models.WebAuthnCredential), "id", id)
}

func (*ormPasskeyRepo) CountByUser(userID string) (int64, error) {
	return orm.NewOrm().QueryTable(new(models.WebAuthnCredential)).Filter("user_id", userID).Count()
}

func (*ormPasskeyRepo) CountByUsers(userIDs []string) (map[string]int64, error) {
	counts := make(map[string]int64, len(userIDs))
	if len(userIDs) == 0 {
		return counts, nil
	}
	var rows []*models.WebAuthnCredential
	if _, err := orm.NewOrm().QueryTable(new(models.WebAuthnCredential)).Filter("user_id__in", userIDs).
		All(&rows, "ID", "UserID"); err != nil {
		return nil, err
	}
	for _, r := range rows {
		counts[r.UserID]++
	}
	return counts, nil
}

func (*ormPasskeyRepo) DeleteByUser(userID string) error {
	return deleteWhere(new(models.WebAuthnCredential), "user_id", userID)
}

// ---- audit ----

type ormAuditRepo struct{}

func (*ormAuditRepo) Create(l *models.AuditLog) error { return insert(l) }

func (*ormAuditRepo) List(userID string, offset, limit int) ([]*models.AuditLog, int64, error) {
	qs := orm.NewOrm().QueryTable(new(models.AuditLog))
	if userID != "" {
		qs = qs.Filter("user_id", userID)
	}
	total, err := qs.Count()
	if err != nil {
		return nil, 0, err
	}
	var logs []*models.AuditLog
	_, err = qs.OrderBy("-created_at", "-id").Limit(limit, offset).All(&logs)
	return logs, total, err
}

// ---- external identities ----

type ormIdentityRepo struct{}

func (*ormIdentityRepo) Create(i *models.UserIdentity) error { return insert(i) }

func (*ormIdentityRepo) GetByProviderSubject(provider, subject string) (*models.UserIdentity, error) {
	i := &models.UserIdentity{}
	return i, one(orm.NewOrm().QueryTable(i).Filter("provider", provider).Filter("subject", subject), i)
}

func (*ormIdentityRepo) ProvidersByUsers(userIDs []string) (map[string][]string, error) {
	out := make(map[string][]string, len(userIDs))
	if len(userIDs) == 0 {
		return out, nil
	}
	var rows []*models.UserIdentity
	if _, err := orm.NewOrm().QueryTable(new(models.UserIdentity)).Filter("user_id__in", userIDs).
		All(&rows, "UserID", "Provider"); err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, r := range rows {
		if k := r.UserID + "|" + r.Provider; !seen[k] {
			seen[k] = true
			out[r.UserID] = append(out[r.UserID], r.Provider)
		}
	}
	for _, p := range out {
		sort.Strings(p)
	}
	return out, nil
}

func (*ormIdentityRepo) DeleteByUser(userID string) error {
	return deleteWhere(new(models.UserIdentity), "user_id", userID)
}
