package services

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/lingster/go-rest-template/backend/apperr"
	"github.com/lingster/go-rest-template/backend/dto"
	"github.com/lingster/go-rest-template/backend/models"
	"github.com/lingster/go-rest-template/backend/repositories"
)

// UserService handles profile reads/updates and DTO presentation.
type UserService struct {
	users    repositories.UserRepository
	passkeys repositories.WebAuthnCredentialRepository
	idents   repositories.IdentityRepository
	audit    *AuditService
}

func NewUserService(users repositories.UserRepository, passkeys repositories.WebAuthnCredentialRepository,
	idents repositories.IdentityRepository, audit *AuditService) *UserService {
	return &UserService{users: users, passkeys: passkeys, idents: idents, audit: audit}
}

// Get loads a user, mapping missing rows to not_found.
func (s *UserService) Get(id string) (*models.User, error) {
	u, err := s.users.GetByID(id)
	if errors.Is(err, repositories.ErrNotFound) {
		return nil, apperr.NotFound("User not found")
	}
	return u, err
}

func toDTO(u *models.User, passkeys int64, identities []string) dto.User {
	if identities == nil {
		identities = []string{}
	}
	return dto.User{HasPassword: u.HasPassword(), Identities: identities,
		ID: u.ID, Email: u.Email, Name: u.Name, Role: u.Role, EmailVerified: u.EmailVerified,
		IsActive: u.IsActive, TOTPEnabled: u.TOTPEnabled, PasskeyCount: int(passkeys),
		CreatedAt: formatTime(u.CreatedAt), UpdatedAt: formatTime(u.UpdatedAt),
	}
}

// Present converts a user to its API shape.
func (s *UserService) Present(u *models.User) (dto.User, error) {
	n, err := s.passkeys.CountByUser(u.ID)
	if err != nil {
		return dto.User{}, err
	}
	prov, err := s.idents.ProvidersByUsers([]string{u.ID})
	if err != nil {
		return dto.User{}, err
	}
	return toDTO(u, n, prov[u.ID]), nil
}

// PresentMany converts several users with a single passkey-count query.
func (s *UserService) PresentMany(users []*models.User) ([]dto.User, error) {
	ids := make([]string, len(users))
	for i, u := range users {
		ids[i] = u.ID
	}
	counts, err := s.passkeys.CountByUsers(ids)
	if err != nil {
		return nil, err
	}
	prov, err := s.idents.ProvidersByUsers(ids)
	if err != nil {
		return nil, err
	}
	out := make([]dto.User, len(users))
	for i, u := range users {
		out[i] = toDTO(u, counts[u.ID], prov[u.ID])
	}
	return out, nil
}

// UpdateName changes the display name.
func (s *UserService) UpdateName(ctx context.Context, u *models.User, name string) error {
	u.Name = strings.TrimSpace(name)
	u.UpdatedAt = time.Now().UTC()
	if err := s.users.Update(u, "Name", "UpdatedAt"); err != nil {
		return err
	}
	s.audit.Log(ctx, AuditProfileUpdate, u.ID, u.ID, nil)
	return nil
}
