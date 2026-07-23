package profile

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/singa/internal/storage"
)

const kindProfile = "profile"

// Manager stores profiles in the SQLite entities table.
type Manager struct {
	mu sync.Mutex
	db *storage.DB
}

// NewManager creates a Manager backed by the shared DB.
func NewManager(db *storage.DB) *Manager {
	return &Manager{db: db}
}

// ── internal helpers ────────────────────────────────────────────────────────

func (m *Manager) findProfile(id string) (*Profile, error) {
	e, err := m.db.GetEntity(kindProfile, id)
	if err != nil {
		return nil, err
	}
	if e == nil {
		return nil, nil
	}
	var p Profile
	if err := json.Unmarshal([]byte(e.Data), &p); err != nil {
		return nil, fmt.Errorf("unmarshal profile %s: %w", id, err)
	}
	return &p, nil
}

func (m *Manager) saveProfile(p *Profile) error {
	return m.db.UpsertEntity(kindProfile, p.ID, p)
}

// ── Public API ──────────────────────────────────────────────────────────────

// List returns all profiles ordered by creation time (oldest first).
func (m *Manager) List() []*Profile {
	m.mu.Lock()
	defer m.mu.Unlock()

	entities, err := m.db.ListEntities(kindProfile)
	if err != nil {
		return []*Profile{}
	}
	out := make([]*Profile, 0, len(entities))
	for _, e := range entities {
		var p Profile
		if err := json.Unmarshal([]byte(e.Data), &p); err != nil {
			continue
		}
		out = append(out, &p)
	}
	return out
}

// Add creates a new profile.
func (m *Manager) Add(name, subscriptionID string, wizardConfig json.RawMessage) (*Profile, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	p := &Profile{
		ID:             uuid.New().String(),
		Name:           name,
		SubscriptionID: subscriptionID,
		UpdatedAt:      time.Now(),
		WizardConfig:   wizardConfig,
	}
	if err := m.saveProfile(p); err != nil {
		return nil, err
	}
	return p, nil
}

// Update replaces name, subscriptionID, and wizardConfig for an existing profile.
func (m *Manager) Update(id, name, subscriptionID string, wizardConfig json.RawMessage) (*Profile, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	p, err := m.findProfile(id)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, fmt.Errorf("profile %q not found", id)
	}
	p.Name = name
	p.SubscriptionID = subscriptionID
	p.UpdatedAt = time.Now()
	if wizardConfig != nil {
		p.WizardConfig = wizardConfig
	}
	if err := m.saveProfile(p); err != nil {
		return nil, err
	}
	return p, nil
}

// Delete removes a profile.
func (m *Manager) Delete(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	p, err := m.findProfile(id)
	if err != nil {
		return err
	}
	if p == nil {
		return fmt.Errorf("profile %q not found", id)
	}
	return m.db.DeleteEntity(kindProfile, id)
}

// GetByID returns a single profile.
func (m *Manager) GetByID(id string) *Profile {
	m.mu.Lock()
	defer m.mu.Unlock()

	p, _ := m.findProfile(id)
	return p
}
