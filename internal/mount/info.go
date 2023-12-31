package mount

import (
	"fmt"
	"regexp"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/openshift/assisted-installer-agent/src/util"
	"github.com/project-flotta/flotta-operator/models"
)

const (
	// mountFinder try to match a line from _mount_ command output
	mountFinder = "(?P<dev>[a-z0-9-\\.\\/_]+)\\s+\\w+\\s+(?P<dst>[a-z0-9-\\.\\/_]+)\\s+\\w+\\s+(?P<type>[a-z0-9-\\._]+).*(?P<opts>\\(.*\\))"
)

func IsPathMounted(path string) bool {
	_, res, err := GetMounts(util.NewDependencies("/"))
	if err != nil {
		return false
	}

	_, found := res[path]
	return found
}

// GetMounts return a list of all host mounts, a map having the directory as key and error if any.
// The map is returned to avoid O(n^2) while trying to match new mounts with the existing ones.
func GetMounts(dep util.IDependencies) ([]*models.Mount, map[string]*models.Mount, error) {
	re, err := regexp.Compile(mountFinder)
	if err != nil {
		return []*models.Mount{}, map[string]*models.Mount{}, fmt.Errorf("failed to compile pattern '%s': %w", mountFinder, err)
	}

	entries, err := list(dep)
	if err != nil {
		return []*models.Mount{}, map[string]*models.Mount{}, err
	}

	mounts := make([]*models.Mount, 0, len(entries))
	mountsMap := make(map[string]*models.Mount)

	for _, entry := range entries {
		if len(entry) == 0 {
			continue
		}
		m := parse(re, entry)
		if m == nil {
			log.Warnf("Cannot parse mount entry '%s'", entry)
			continue
		}

		mounts = append(mounts, m)
		mountsMap[m.Directory] = m
	}

	return mounts, mountsMap, nil
}

func list(dep util.IDependencies) ([]string, error) {
	stout, stderr, exitCode := dep.Execute("mount")
	if exitCode != 0 {
		return []string{}, fmt.Errorf("failed to list mounts: %s", stderr)
	}

	return strings.Split(stout, "\n"), nil
}

func parse(re *regexp.Regexp, entry string) *models.Mount {
	groups := re.SubexpNames()

	match := re.FindAllStringSubmatch(entry, -1)
	if match == nil {
		return nil
	}

	mount := models.Mount{}
	for _, m := range match {
		for idx, val := range m {
			switch groups[idx] {
			case "dev":
				mount.Device = val
			case "dst":
				mount.Directory = val
			case "type":
				mount.Type = val
			case "opts":
				mount.Options = val
			}
		}
	}

	return &mount
}
