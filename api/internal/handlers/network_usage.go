package handlers

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"cloudpass/internal/logger"
	"cloudpass/internal/models"
)

// NetworkUsageStore tracks which multipass network each instance uses and
// which networks CloudPass itself created. Multipass offers no query for
// either fact (instance info keys interfaces by guest name, and the
// networks list mixes created bridges with host/docker/daemon ones), so
// both are recorded at the moment they become known: launch, create,
// manual attribution, and deletion.
//
// Anything outside these records is explicitly unknown, never guessed.
type NetworkUsageStore struct {
	filePath string
	mu       sync.RWMutex
	// instanceNetworks maps instance name -> multipass network name.
	// Absence means the attachment is unknown.
	instanceNetworks map[string]string
	// managedNetworks is the set of network names CloudPass created.
	// Delete is only offered for these; host, docker, and daemon
	// bridges stay undeletable by construction.
	managedNetworks map[string]bool
}

type networkUsageFile struct {
	InstanceNetworks map[string]string `json:"instance_networks"`
	ManagedNetworks  []string          `json:"managed_networks"`
}

func NewNetworkUsageStore(dataDir string) (*NetworkUsageStore, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	storage := &NetworkUsageStore{
		filePath:         filepath.Join(dataDir, "instance_networks.json"),
		instanceNetworks: make(map[string]string),
		managedNetworks:  make(map[string]bool),
	}

	if err := storage.load(); err != nil {
		logger.API.Load().Warn().Err(err).Msg("failed to load network usage file, starting fresh")
	}

	return storage, nil
}

func (s *NetworkUsageStore) load() error {
	data, err := os.ReadFile(s.filePath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}

	var file networkUsageFile
	if err := json.Unmarshal(data, &file); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	for instance, network := range file.InstanceNetworks {
		if instance != "" && network != "" {
			s.instanceNetworks[instance] = network
		}
	}
	for _, name := range file.ManagedNetworks {
		if name != "" {
			s.managedNetworks[name] = true
		}
	}
	return nil
}

func (s *NetworkUsageStore) save() {
	file := networkUsageFile{
		InstanceNetworks: make(map[string]string, len(s.instanceNetworks)),
		ManagedNetworks:  make([]string, 0, len(s.managedNetworks)),
	}
	for instance, network := range s.instanceNetworks {
		file.InstanceNetworks[instance] = network
	}
	for name := range s.managedNetworks {
		file.ManagedNetworks = append(file.ManagedNetworks, name)
	}
	sort.Strings(file.ManagedNetworks)

	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		logger.API.Load().Error().Err(err).Msg("failed to marshal network usage file")
		return
	}
	if err := os.WriteFile(s.filePath, data, 0644); err != nil {
		logger.API.Load().Error().Err(err).Msg("failed to persist network usage file")
	}
}

// RecordInstanceNetwork remembers that instance uses network. Empty
// networks (default launch) are not recorded: absence means unknown,
// never "default".
func (s *NetworkUsageStore) RecordInstanceNetwork(instance, network string) {
	if instance == "" || network == "" {
		return
	}
	s.mu.Lock()
	s.instanceNetworks[instance] = network
	s.save()
	s.mu.Unlock()
}

// ClearInstanceNetwork forgets any recorded attachment, e.g. on
// instance delete or manual clear.
func (s *NetworkUsageStore) ClearInstanceNetwork(instance string) {
	s.mu.Lock()
	delete(s.instanceNetworks, instance)
	s.save()
	s.mu.Unlock()
}

// InstanceNetwork returns the recorded network for instance, if any.
func (s *NetworkUsageStore) InstanceNetwork(instance string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	network, ok := s.instanceNetworks[instance]
	return network, ok
}

// RecordCreatedNetwork marks name as CloudPass-managed after a
// successful create.
func (s *NetworkUsageStore) RecordCreatedNetwork(name string) {
	if name == "" {
		return
	}
	s.mu.Lock()
	s.managedNetworks[name] = true
	s.save()
	s.mu.Unlock()
}

// RemoveCreatedNetwork drops the managed mark after a successful delete.
func (s *NetworkUsageStore) RemoveCreatedNetwork(name string) {
	s.mu.Lock()
	delete(s.managedNetworks, name)
	s.save()
	s.mu.Unlock()
}

// IsManaged reports whether name was created by CloudPass (and is still
// known to exist). Only managed networks are offered for deletion.
func (s *NetworkUsageStore) IsManaged(name string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.managedNetworks[name]
}

// PruneManaged drops managed marks for networks no longer present,
// e.g. deleted out-of-band. Call with the current listed names.
func (s *NetworkUsageStore) PruneManaged(present []string) {
	keep := make(map[string]bool, len(present))
	for _, name := range present {
		keep[name] = true
	}
	s.mu.Lock()
	changed := false
	for name := range s.managedNetworks {
		if !keep[name] {
			delete(s.managedNetworks, name)
			changed = true
		}
	}
	if changed {
		s.save()
	}
	s.mu.Unlock()
}

// trackedInstances returns the names of all instances with a recorded
// attachment. Used to prune records for deleted instances.
func (s *NetworkUsageStore) trackedInstances() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	names := make([]string, 0, len(s.instanceNetworks))
	for instance := range s.instanceNetworks {
		names = append(names, instance)
	}
	return names
}

// UsageReport splits instances into confirmed users per network and
// unverified names. An untracked instance is safe to ignore only when it
// is Running with at most one IPv4 (the default interface); anything
// else (extra interfaces, stopped/unknown state with empty info) may use
// a custom network and must be treated as unknown. Outputs are sorted.
func (s *NetworkUsageStore) UsageReport(instances []models.Instance) (map[string][]string, []string) {
	usedBy := make(map[string][]string)
	var unverified []string

	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, instance := range instances {
		if network, ok := s.instanceNetworks[instance.Name]; ok {
			usedBy[network] = append(usedBy[network], instance.Name)
			continue
		}
		if instance.State == "Running" && len(instance.IPv4) <= 1 {
			continue
		}
		unverified = append(unverified, instance.Name)
	}

	for network := range usedBy {
		sort.Strings(usedBy[network])
	}
	sort.Strings(unverified)
	return usedBy, unverified
}
