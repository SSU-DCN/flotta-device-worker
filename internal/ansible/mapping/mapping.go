package mapping

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"sort"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

type mapping struct {
	ModTime  int64  `json:"mod_time"` //unix nano
	FilePath string `json:"file_path"`
	Name     string `json:"name"`
	Status   string `json:"status"`
}

//go:generate mockgen -package=mapping -destination=mock_mapping.go . MappingRepository
type MappingRepository interface {
	GetSha256(fileContent []byte) string
	Add(peName string, fileContent []byte, modTime time.Time, status string) error
	Remove(fileContent []byte) error
	RemoveMappingFile() error
	GetModTime(filePath string) int64
	GetFilePath(modTime time.Time) string
	GetName(filePath string) string
	Persist() error
	Size() int
	Exists(peName string) bool
	GetAll() map[int]string
	GetAllNames() []string
	GetStatus(name string) string
	GetAllNamesStatus() map[string]string
	UpdateStatus(name, status string) error
}

type mappingRepository struct {
	mappingFilePath string
	modTimeToPath   map[int64]string
	pathToModTime   map[string]int64
	pathToName      map[string]string
	nameStatus      map[string]string
	lock            sync.RWMutex
	configDir       string
	name            string
}

func NewMappingRepository(configDir string) (MappingRepository, error) {

	mappingFilePath := path.Join(configDir, "playbook-mapping.json")

	mappingJSON, err := os.ReadFile(mappingFilePath) //#nosec
	var mappings []mapping
	if err == nil {
		err := json.Unmarshal(mappingJSON, &mappings)
		if err != nil {
			return nil, err
		}
	}

	modTimeToPath := make(map[int64]string)
	pathToModTime := make(map[string]int64)
	pathToName := make(map[string]string)
	nameStatus := make(map[string]string)

	for _, mapping := range mappings {
		modTimeToPath[mapping.ModTime] = mapping.FilePath
		pathToModTime[mapping.FilePath] = mapping.ModTime
		pathToName[mapping.FilePath] = mapping.Name
		nameStatus[mapping.Name] = mapping.Status
	}

	return &mappingRepository{
		mappingFilePath: mappingFilePath,
		lock:            sync.RWMutex{},
		modTimeToPath:   modTimeToPath,
		pathToModTime:   pathToModTime,
		pathToName:      pathToName,
		nameStatus:      nameStatus,
		configDir:       configDir,
	}, nil
}

func (m *mappingRepository) GetAll() map[int]string {
	m.lock.RLock()
	defer m.lock.RUnlock()

	all := make(map[int]string)
	keys := make([]int64, 0, len(m.modTimeToPath))
	for k := range m.modTimeToPath {
		keys = append(keys, k)
	}

	sort.SliceStable(keys, func(i, j int) bool { return keys[i] < keys[j] })

	for i, v := range keys {
		all[i] = m.modTimeToPath[v]
	}

	return all
}

func (m *mappingRepository) GetSha256(fileContent []byte) string {
	h := sha256.New()
	h.Write(fileContent)
	return fmt.Sprintf("%x", h.Sum(nil))
}

func (m *mappingRepository) Add(peName string, fileContent []byte, modTime time.Time, status string) error {
	m.lock.Lock()
	defer m.lock.Unlock()
	filePath := path.Join(m.configDir, m.GetSha256(fileContent))
	err := os.WriteFile(filePath, []byte(fileContent), 0600)

	if err != nil {
		return err
	}
	m.name = peName
	m.modTimeToPath[modTime.UnixNano()] = filePath
	m.pathToModTime[filePath] = modTime.UnixNano()
	m.pathToName[filePath] = peName
	m.nameStatus[peName] = status

	return m.persist()
}

func (m *mappingRepository) Remove(fileContent []byte) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	filePath := path.Join(m.configDir, m.GetSha256(fileContent))
	modTime := m.pathToModTime[filePath]
	delete(m.modTimeToPath, modTime)
	delete(m.pathToModTime, filePath)
	delete(m.pathToName, filePath)
	return m.persist()
}

func (m *mappingRepository) RemoveMappingFile() error {
	m.lock.Lock()
	defer m.lock.Unlock()

	log.Infof("deleting %s file", m.mappingFilePath)
	err := os.RemoveAll(m.mappingFilePath)
	if err != nil {
		log.Errorf("failed to delete %s: %v", m.mappingFilePath, err)
		return err
	}

	return nil
}

func (m *mappingRepository) GetModTime(filePath string) int64 {
	m.lock.RLock()
	defer m.lock.RUnlock()

	return m.pathToModTime[filePath]
}

func (m *mappingRepository) GetFilePath(modTime time.Time) string {
	m.lock.RLock()
	defer m.lock.RUnlock()

	return m.modTimeToPath[modTime.UnixNano()]
}

func (m *mappingRepository) GetName(filePath string) string {
	m.lock.RLock()
	defer m.lock.RUnlock()

	return m.pathToName[filePath]
}

func (m *mappingRepository) GetAllNames() []string {
	m.lock.RLock()
	defer m.lock.RUnlock()

	names := make([]string, 0, len(m.pathToName))
	for _, val := range m.pathToName {
		names = append(names, val)
	}

	return names
}

func (m *mappingRepository) GetAllNamesStatus() map[string]string {
	m.lock.RLock()
	defer m.lock.RUnlock()

	return m.nameStatus
}

func (m *mappingRepository) GetStatus(name string) string {
	m.lock.RLock()
	defer m.lock.RUnlock()

	return m.nameStatus[name]
}

func (m *mappingRepository) UpdateStatus(name, status string) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	if _, ok := m.nameStatus[name]; !ok {
		return fmt.Errorf("cannot find playbook execution with name: %s", name)
	}

	m.nameStatus[name] = status
	return nil

}

func (m *mappingRepository) Exists(peName string) bool {
	m.lock.RLock()
	defer m.lock.RUnlock()

	_, x := m.nameStatus[peName]
	return x
}

func (m *mappingRepository) Persist() error {
	m.lock.RLock()
	defer m.lock.RUnlock()
	return m.persist()
}

func (m *mappingRepository) Size() int {
	m.lock.RLock()
	defer m.lock.RUnlock()

	return len(m.pathToModTime)
}

func (m *mappingRepository) persist() error {
	var mappings []mapping

	for modTime, path := range m.modTimeToPath {
		mappings = append(mappings, mapping{ModTime: modTime, FilePath: path, Name: m.pathToName[path], Status: m.nameStatus[m.pathToName[path]]})
	}
	mappingsJSON, err := json.Marshal(mappings)
	if err != nil {
		return err
	}
	err = os.WriteFile(m.mappingFilePath, mappingsJSON, 0640) //#nosec
	if err != nil {
		return err
	}
	return nil
}
