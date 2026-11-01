package services

import (
	"context"
	"errors"
	"log"
	"strconv"
	"time"

	"github.com/lingster/go-rest-template/backend/apperr"
	"github.com/lingster/go-rest-template/backend/dto"
	"github.com/lingster/go-rest-template/backend/models"
	"github.com/lingster/go-rest-template/backend/repositories"
)

// AdminService implements admin-only user management.
type AdminService struct {
	repos   *repositories.Set
	hasher  PasswordHasher
	tokens  *TokenService
	userSvc *UserService
	audit   *AuditService
}

func NewAdminService(repos *repositories.Set, hasher PasswordHasher, tokens *TokenService,
	userSvc *UserService, audit *AuditService) *AdminService {
	return &AdminService{repos: repos, hasher: hasher, tokens: tokens, userSvc: userSvc, audit: audit}
}

// Stats returns dashboard counters.
func (s *AdminService) Stats() (*dto.Stats, error) {
	st, err := s.repos.Users.Stats()
	if err != nil {
		return nil, err
	}
	return &dto.Stats{UsersTotal: st.Total, UsersActive: st.Active, UsersVerified: st.Verified, Users2FA: st.TwoFA}, nil
}

// ListUsers returns a page of users matching q (email/name substring).
func (s *AdminService) ListUsers(q string, page, perPage int) (*dto.UserPage, error) {
	page, perPage = NormalizePaging(page, perPage)
	users, total, err := s.repos.Users.List(q, (page-1)*perPage, perPage)
	if err != nil {
		return nil, err
	}
	items, err := s.userSvc.PresentMany(users)
	if err != nil {
		return nil, err
	}
	return &dto.UserPage{Items: items, Total: total, Page: page, PerPage: perPage}, nil
}

// GetUser returns one user.
func (s *AdminService) GetUser(id string) (*dto.User, error) {
	u, err := s.userSvc.Get(id)
	if err != nil {
		return nil, err
	}
	v, err := s.userSvc.Present(u)
	return &v, err
}

// UpdateUser applies a partial update. Admins cannot demote or deactivate themselves.
func (s *AdminService) UpdateUser(ctx context.Context, actor *models.User, id string, req *dto.AdminUpdateUserRequest) (*dto.User, error) {
	u, err := s.userSvc.Get(id)
	if err != nil {
		return nil, err
	}
	self := actor.ID == u.ID
	changes := map[string]string{}
	var cols []string
	if req.Name != nil {
		u.Name = *req.Name
		cols = append(cols, "Name")
		changes["name"] = *req.Name
	}
	if req.Role != nil && *req.Role != u.Role {
		if self {
			return nil, apperr.Forbidden("You cannot change your own role")
		}
		u.Role = *req.Role
		cols = append(cols, "Role")
		changes["role"] = *req.Role
	}
	if req.IsActive != nil && *req.IsActive != u.IsActive {
		if self {
			return nil, apperr.Forbidden("You cannot deactivate yourself")
		}
		u.IsActive = *req.IsActive
		cols = append(cols, "IsActive")
		changes["is_active"] = strconv.FormatBool(u.IsActive)
	}
	if req.EmailVerified != nil && *req.EmailVerified != u.EmailVerified {
		u.EmailVerified = *req.EmailVerified
		cols = append(cols, "EmailVerified")
		changes["email_verified"] = strconv.FormatBool(u.EmailVerified)
	}
	if len(cols) > 0 {
		u.UpdatedAt = time.Now().UTC()
		if err := s.repos.Users.Update(u, append(cols, "UpdatedAt")...); err != nil {
			return nil, err
		}
		if req.IsActive != nil && !u.IsActive {
			_ = s.tokens.RevokeAll(u.ID)
		}
		s.audit.Log(ctx, AuditAdminUserUpdate, u.ID, actor.ID, changes)
	}
	v, err := s.userSvc.Present(u)
	return &v, err
}

// DeleteUser removes a user and their credentials/tokens. Audit entries are kept.
func (s *AdminService) DeleteUser(ctx context.Context, actor *models.User, id string) error {
	if actor.ID == id {
		return apperr.Forbidden("You cannot delete yourself")
	}
	u, err := s.userSvc.Get(id)
	if err != nil {
		return err
	}
	for _, del := range []func(string) error{
		s.repos.RefreshTokens.DeleteByUser, s.repos.OneTime.DeleteByUser, s.repos.Recovery.DeleteByUser,
		s.repos.Passkeys.DeleteByUser, s.repos.Users.Delete,
	} {
		if err := del(u.ID); err != nil {
			return err
		}
	}
	s.audit.Log(ctx, AuditAdminUserDelete, u.ID, actor.ID, map[string]string{"email": u.Email})
	return nil
}

// RevokeSessions revokes every refresh token of the user.
func (s *AdminService) RevokeSessions(ctx context.Context, actor *models.User, id string) error {
	if _, err := s.userSvc.Get(id); err != nil {
		return err
	}
	if err := s.tokens.RevokeAll(id); err != nil {
		return err
	}
	s.audit.Log(ctx, AuditAdminRevokeSess, id, actor.ID, nil)
	return nil
}

// ListAuditLogs returns audit entries.
func (s *AdminService) ListAuditLogs(userID string, page, perPage int) (*dto.AuditLogPage, error) {
	return s.audit.List(userID, page, perPage)
}

// SeedAdmin creates a verified admin if none exists with the email.
func (s *AdminService) SeedAdmin(ctx context.Context, email, password string) error {
	email = dto.NormalizeEmail(email)
	if email == "" || password == "" {
		return nil
	}
	if _, err := s.repos.Users.GetByEmail(email); err == nil {
		return nil
	} else if !errors.Is(err, repositories.ErrNotFound) {
		return err
	}
	hash, err := s.hasher.Hash(password)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	u := &models.User{ID: NewID(), Email: email, Name: "Admin", PasswordHash: hash, Role: models.RoleAdmin,
		EmailVerified: true, IsActive: true, CreatedAt: now, UpdatedAt: now}
	if err := s.repos.Users.Create(u); err != nil && !errors.Is(err, repositories.ErrDuplicate) {
		return err
	}
	s.audit.Log(ctx, AuditAdminSeedCreated, u.ID, "", nil)
	log.Printf("seeded admin user %s", email)
	return nil
}
