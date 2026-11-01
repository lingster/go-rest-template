package services

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"

	"github.com/lingster/go-rest-template/backend/apperr"
	"github.com/lingster/go-rest-template/backend/config"
	"github.com/lingster/go-rest-template/backend/dto"
	"github.com/lingster/go-rest-template/backend/models"
	"github.com/lingster/go-rest-template/backend/repositories"
)

// CeremonyTTL is how long a WebAuthn ceremony session stays valid.
const CeremonyTTL = 5 * time.Minute

// Ceremony kinds.
const (
	CeremonyRegister = "register"
	CeremonyLogin    = "login"
)

// CeremonySession is the server-side state of an in-flight WebAuthn ceremony.
type CeremonySession struct {
	Kind   string
	UserID string // empty for discoverable login
	Data   webauthn.SessionData
}

// SessionStore keeps ceremony sessions. Swap the in-memory version for Redis/DB to run multiple instances.
type SessionStore interface {
	Put(id string, s CeremonySession, ttl time.Duration)
	// Take returns and deletes the session (single use).
	Take(id string) (CeremonySession, bool)
}

type memEntry struct {
	s       CeremonySession
	expires time.Time
}

// MemorySessionStore is a TTL map implementing SessionStore.
type MemorySessionStore struct {
	mu sync.Mutex
	m  map[string]memEntry
}

func NewMemorySessionStore() *MemorySessionStore {
	return &MemorySessionStore{m: map[string]memEntry{}}
}

func (s *MemorySessionStore) Put(id string, cs CeremonySession, ttl time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for k, e := range s.m { // opportunistic sweep keeps the map bounded
		if now.After(e.expires) {
			delete(s.m, k)
		}
	}
	s.m[id] = memEntry{s: cs, expires: now.Add(ttl)}
}

func (s *MemorySessionStore) Take(id string) (CeremonySession, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.m[id]
	delete(s.m, id)
	if !ok || time.Now().After(e.expires) {
		return CeremonySession{}, false
	}
	return e.s, true
}

// waUser adapts models.User to webauthn.User.
type waUser struct {
	u     *models.User
	creds []webauthn.Credential
}

func (w *waUser) WebAuthnID() []byte                         { return []byte(w.u.ID) }
func (w *waUser) WebAuthnName() string                       { return w.u.Email }
func (w *waUser) WebAuthnDisplayName() string                { return w.u.Name }
func (w *waUser) WebAuthnCredentials() []webauthn.Credential { return w.creds }

// PasskeyService implements passkey registration and login ceremonies.
type PasskeyService struct {
	wa       *webauthn.WebAuthn
	repo     repositories.WebAuthnCredentialRepository
	users    repositories.UserRepository
	sessions SessionStore
	audit    *AuditService
}

func NewPasskeyService(cfg *config.Config, repo repositories.WebAuthnCredentialRepository,
	users repositories.UserRepository, sessions SessionStore, audit *AuditService) (*PasskeyService, error) {
	wa, err := webauthn.New(&webauthn.Config{
		RPID: cfg.WebAuthnRPID, RPDisplayName: cfg.WebAuthnRPName, RPOrigins: cfg.WebAuthnRPOrigins,
	})
	if err != nil {
		return nil, err
	}
	return &PasskeyService{wa: wa, repo: repo, users: users, sessions: sessions, audit: audit}, nil
}

func (s *PasskeyService) load(u *models.User) (*waUser, error) {
	rows, err := s.repo.ListByUser(u.ID)
	if err != nil {
		return nil, err
	}
	w := &waUser{u: u}
	for _, r := range rows {
		var c webauthn.Credential
		if err := json.Unmarshal([]byte(r.CredentialJSON), &c); err == nil {
			w.creds = append(w.creds, c)
		}
	}
	return w, nil
}

func encodeCredID(id []byte) string { return base64.RawURLEncoding.EncodeToString(id) }

// List returns the user's passkeys.
func (s *PasskeyService) List(userID string) ([]dto.Passkey, error) {
	rows, err := s.repo.ListByUser(userID)
	if err != nil {
		return nil, err
	}
	out := make([]dto.Passkey, 0, len(rows))
	for _, r := range rows {
		out = append(out, presentPasskey(r))
	}
	return out, nil
}

func presentPasskey(r *models.WebAuthnCredential) dto.Passkey {
	p := dto.Passkey{ID: r.ID, Name: r.Name, CreatedAt: formatTime(r.CreatedAt)}
	if r.LastUsedAt != nil {
		t := formatTime(*r.LastUsedAt)
		p.LastUsedAt = &t
	}
	return p
}

// Delete removes one of the user's passkeys.
func (s *PasskeyService) Delete(ctx context.Context, userID, id string) error {
	c, err := s.repo.GetForUser(id, userID)
	if errors.Is(err, repositories.ErrNotFound) {
		return apperr.NotFound("Passkey not found")
	}
	if err != nil {
		return err
	}
	if err := s.repo.Delete(c.ID); err != nil {
		return err
	}
	s.audit.Log(ctx, AuditPasskeyRemove, userID, userID, map[string]string{"passkey_id": id})
	return nil
}

// BeginRegistration starts adding a passkey for the user.
func (s *PasskeyService) BeginRegistration(u *models.User) (string, *protocol.CredentialCreation, error) {
	w, err := s.load(u)
	if err != nil {
		return "", nil, err
	}
	excl := make([]protocol.CredentialDescriptor, 0, len(w.creds))
	for i := range w.creds {
		excl = append(excl, w.creds[i].Descriptor())
	}
	opts, session, err := s.wa.BeginRegistration(w,
		webauthn.WithResidentKeyRequirement(protocol.ResidentKeyRequirementRequired),
		webauthn.WithExclusions(excl))
	if err != nil {
		return "", nil, err
	}
	id := RandomToken(24)
	s.sessions.Put(id, CeremonySession{Kind: CeremonyRegister, UserID: u.ID, Data: *session}, CeremonyTTL)
	return id, opts, nil
}

// FinishRegistration verifies the attestation and stores the credential.
func (s *PasskeyService) FinishRegistration(ctx context.Context, u *models.User, sessionID, name string, credJSON []byte) (*dto.Passkey, error) {
	cs, ok := s.sessions.Take(sessionID)
	if !ok || cs.Kind != CeremonyRegister || cs.UserID != u.ID {
		return nil, apperr.InvalidToken("Unknown or expired passkey session")
	}
	parsed, err := protocol.ParseCredentialCreationResponseBytes(credJSON)
	if err != nil {
		return nil, apperr.Validation(map[string]string{"credential": "malformed credential"})
	}
	w, err := s.load(u)
	if err != nil {
		return nil, err
	}
	cred, err := s.wa.CreateCredential(w, cs.Data, parsed)
	if err != nil {
		return nil, apperr.Validation(map[string]string{"credential": "registration could not be verified"})
	}
	raw, err := json.Marshal(cred)
	if err != nil {
		return nil, err
	}
	if name = strings.TrimSpace(name); name == "" {
		name = "Passkey"
	}
	row := &models.WebAuthnCredential{
		ID: NewID(), UserID: u.ID, Name: name, CredentialID: encodeCredID(cred.ID),
		CredentialJSON: string(raw), CreatedAt: time.Now().UTC(),
	}
	if err := s.repo.Create(row); errors.Is(err, repositories.ErrDuplicate) {
		return nil, apperr.Conflict("Passkey already registered")
	} else if err != nil {
		return nil, err
	}
	s.audit.Log(ctx, AuditPasskeyAdd, u.ID, u.ID, map[string]string{"passkey_id": row.ID})
	p := presentPasskey(row)
	return &p, nil
}

// BeginLogin starts a passkey login; email "" starts a discoverable-credential login.
func (s *PasskeyService) BeginLogin(email string) (string, *protocol.CredentialAssertion, error) {
	var (
		opts    *protocol.CredentialAssertion
		session *webauthn.SessionData
		userID  string
		err     error
	)
	if email == "" {
		opts, session, err = s.wa.BeginDiscoverableLogin()
	} else {
		u, uerr := s.users.GetByEmail(email)
		if errors.Is(uerr, repositories.ErrNotFound) {
			return "", nil, apperr.InvalidCredentials()
		} else if uerr != nil {
			return "", nil, uerr
		}
		w, lerr := s.load(u)
		if lerr != nil {
			return "", nil, lerr
		}
		if len(w.creds) == 0 {
			return "", nil, apperr.InvalidCredentials()
		}
		userID = u.ID
		opts, session, err = s.wa.BeginLogin(w)
	}
	if err != nil {
		return "", nil, err
	}
	id := RandomToken(24)
	s.sessions.Put(id, CeremonySession{Kind: CeremonyLogin, UserID: userID, Data: *session}, CeremonyTTL)
	return id, opts, nil
}

// FinishLogin verifies the assertion and returns the authenticated user.
func (s *PasskeyService) FinishLogin(sessionID string, credJSON []byte) (*models.User, error) {
	cs, ok := s.sessions.Take(sessionID)
	if !ok || cs.Kind != CeremonyLogin {
		return nil, apperr.InvalidToken("Unknown or expired passkey session")
	}
	parsed, err := protocol.ParseCredentialRequestResponseBytes(credJSON)
	if err != nil {
		return nil, apperr.Validation(map[string]string{"credential": "malformed credential"})
	}
	var (
		user *models.User
		cred *webauthn.Credential
	)
	if cs.UserID != "" {
		u, uerr := s.users.GetByID(cs.UserID)
		if uerr != nil {
			return nil, apperr.InvalidCredentials()
		}
		w, lerr := s.load(u)
		if lerr != nil {
			return nil, lerr
		}
		if cred, err = s.wa.ValidateLogin(w, cs.Data, parsed); err != nil {
			return nil, apperr.InvalidCredentials()
		}
		user = u
	} else {
		var wu webauthn.User
		wu, cred, err = s.wa.ValidatePasskeyLogin(func(_, userHandle []byte) (webauthn.User, error) {
			u, uerr := s.users.GetByID(string(userHandle))
			if uerr != nil {
				return nil, uerr
			}
			return s.load(u)
		}, cs.Data, parsed)
		if err != nil {
			return nil, apperr.InvalidCredentials()
		}
		user = wu.(*waUser).u
	}
	s.touch(cred)
	return user, nil
}

// touch persists the updated sign counter and last-used timestamp.
func (s *PasskeyService) touch(cred *webauthn.Credential) {
	row, err := s.repo.GetByCredentialID(encodeCredID(cred.ID))
	if err != nil {
		return
	}
	if raw, err := json.Marshal(cred); err == nil {
		row.CredentialJSON = string(raw)
	}
	now := time.Now().UTC()
	row.LastUsedAt = &now
	_ = s.repo.Update(row, "CredentialJSON", "LastUsedAt")
}
