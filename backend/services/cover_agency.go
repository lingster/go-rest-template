package services

import (
	"context"
	"errors"
	"strings"

	"github.com/lingster/go-rest-template/backend/apperr"
	"github.com/lingster/go-rest-template/backend/dto"
	"github.com/lingster/go-rest-template/backend/models"
	"github.com/lingster/go-rest-template/backend/ratelimit"
	"github.com/lingster/go-rest-template/backend/repositories"
)

// agencyKeyPrefix makes leaked keys recognisable (e.g. by secret scanners).
const agencyKeyPrefix = "agk_"

// maxAgencyKeyLen bounds the header we bother hashing.
const maxAgencyKeyLen = 128

// AgencyService manages agencies, their API keys and supply teachers, and authenticates agency calls.
type AgencyService struct {
	CoverDeps
	// keyFailures budgets invalid API keys per client (IP / IPv6 /64); perAgency limits each valid key.
	keyFailures ratelimit.FailureCounter
	perAgency   ratelimit.Limiter
}

// NewAgencyService wires the service. perAgency may be nil (no per-agency limit).
func NewAgencyService(deps CoverDeps, keyFailures ratelimit.FailureCounter, perAgency ratelimit.Limiter) *AgencyService {
	return &AgencyService{CoverDeps: deps, keyFailures: keyFailures, perAgency: perAgency}
}

func presentAgency(a *models.Agency) dto.Agency {
	return dto.Agency{ID: a.ID, Name: a.Name, Phone: a.Phone, IsActive: a.IsActive, CreatedAt: formatTime(a.CreatedAt)}
}

func presentTeacher(t *models.SupplyTeacher) dto.SupplyTeacher {
	return dto.SupplyTeacher{ID: t.ID, AgencyID: t.AgencyID, Name: t.Name, Subject: t.Subject,
		DBSConfirmed: t.DBSConfirmed, CreatedAt: formatTime(t.CreatedAt)}
}

// newAgencyKey returns a fresh plaintext key and the hash to store.
func newAgencyKey() (key, hash string) {
	key = agencyKeyPrefix + RandomToken(32)
	return key, HashToken(key)
}

// CreateAgency creates an agency and returns its API key. The plaintext key is never stored.
func (s *AgencyService) CreateAgency(ctx context.Context, actor *models.User, req *dto.CreateAgencyRequest) (*dto.AgencyKeyResponse, error) {
	if err := requireCoverManager(actor); err != nil {
		return nil, err
	}
	key, hash := newAgencyKey()
	a := &models.Agency{ID: NewID(), Name: strings.TrimSpace(req.Name), Phone: strings.TrimSpace(req.Phone),
		APIKeyHash: hash, IsActive: true, CreatedAt: s.Now().UTC()}
	if err := s.Repos.Agencies.Create(a); errors.Is(err, repositories.ErrDuplicate) {
		return nil, apperr.Conflict("An agency with this name already exists")
	} else if err != nil {
		return nil, err
	}
	s.Audit.Log(ctx, AuditAgencyCreate, "", actor.ID, map[string]string{"agency_id": a.ID, "name": a.Name})
	return &dto.AgencyKeyResponse{Agency: presentAgency(a), APIKey: key}, nil
}

// ListAgencies returns all agencies (without keys or hashes).
func (s *AgencyService) ListAgencies() ([]dto.Agency, error) {
	list, err := s.Repos.Agencies.List()
	if err != nil {
		return nil, err
	}
	out := make([]dto.Agency, len(list))
	for i, a := range list {
		out[i] = presentAgency(a)
	}
	return out, nil
}

// agency loads an agency, mapping a missing row to not_found.
func (s *AgencyService) agency(id string) (*models.Agency, error) {
	a, err := s.Repos.Agencies.GetByID(id)
	if errors.Is(err, repositories.ErrNotFound) {
		return nil, apperr.NotFound("Agency not found")
	}
	return a, err
}

// RotateKey issues a new API key; the previous one stops working immediately.
func (s *AgencyService) RotateKey(ctx context.Context, actor *models.User, id string) (*dto.AgencyKeyResponse, error) {
	if err := requireCoverManager(actor); err != nil {
		return nil, err
	}
	a, err := s.agency(id)
	if err != nil {
		return nil, err
	}
	key, hash := newAgencyKey()
	if err := s.Repos.Agencies.SetKeyHash(a.ID, hash); err != nil {
		return nil, err
	}
	s.Audit.Log(ctx, AuditAgencyKeyRotate, "", actor.ID, map[string]string{"agency_id": a.ID})
	return &dto.AgencyKeyResponse{Agency: presentAgency(a), APIKey: key}, nil
}

// CreateTeacher registers a supply teacher with an agency.
func (s *AgencyService) CreateTeacher(actor *models.User, req *dto.CreateTeacherRequest) (*dto.SupplyTeacher, error) {
	if err := requireCoverManager(actor); err != nil {
		return nil, err
	}
	if _, err := s.Repos.Agencies.GetByID(req.AgencyID); errors.Is(err, repositories.ErrNotFound) {
		return nil, apperr.Validation(map[string]string{"agency_id": "unknown agency"})
	} else if err != nil {
		return nil, err
	}
	t := &models.SupplyTeacher{ID: NewID(), AgencyID: req.AgencyID, Name: strings.TrimSpace(req.Name),
		Subject: strings.TrimSpace(req.Subject), DBSConfirmed: req.DBSConfirmed, CreatedAt: s.Now().UTC()}
	if err := s.Repos.Teachers.Create(t); err != nil {
		return nil, err
	}
	v := presentTeacher(t)
	return &v, nil
}

// ListTeachers returns supply teachers, optionally of one agency.
func (s *AgencyService) ListTeachers(agencyID string) ([]dto.SupplyTeacher, error) {
	list, err := s.Repos.Teachers.List(agencyID)
	if err != nil {
		return nil, err
	}
	out := make([]dto.SupplyTeacher, len(list))
	for i, t := range list {
		out[i] = presentTeacher(t)
	}
	return out, nil
}

// lookupKey finds the agency owning key (by its SHA-256 hash; oversized keys are never looked up).
func (s *AgencyService) lookupKey(key string) (*models.Agency, error) {
	if len(key) > maxAgencyKeyLen {
		return nil, repositories.ErrNotFound
	}
	return s.Repos.Agencies.GetByKeyHash(HashToken(key))
}

// invalidAgencyKey is deliberately identical for a missing, unknown, rotated-out or inactive key.
func invalidAgencyKey() error { return apperr.Unauthorized("Invalid or missing agency key") }

// Authenticate resolves an agency API key. client is the caller's rate-limit key (ratelimit.ClientKey):
// clients that keep presenting invalid keys are throttled before any lookup, and each valid key is
// limited per minute.
func (s *AgencyService) Authenticate(client, key string) (*models.Agency, error) {
	if s.keyFailures != nil && s.keyFailures.Blocked(client) {
		return nil, apperr.RateLimited()
	}
	if key == "" {
		return nil, invalidAgencyKey()
	}
	a, err := s.lookupKey(key)
	if err == nil && !a.IsActive {
		err = repositories.ErrNotFound
	}
	if errors.Is(err, repositories.ErrNotFound) {
		if s.keyFailures != nil {
			s.keyFailures.Fail(client)
		}
		return nil, invalidAgencyKey()
	} else if err != nil {
		return nil, err
	}
	if s.perAgency != nil && !s.perAgency.Allow(a.ID) {
		return nil, apperr.RateLimited()
	}
	return a, nil
}
