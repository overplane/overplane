package paths

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	configDataDir    = "config/data"
	configSchemaDir  = "config/schema"
	dataMarkerFile   = configDataDir + "/global.yaml"
	schemaMarkerFile = configSchemaDir + "/global.schema.json"
)

// Paths holds every filesystem location that config-aware tools share. All
// paths are absolute and derived from a single discovered repository root.
type Paths struct {
	Root            string
	ConfigDir       string
	ConfigDataDir   string
	ConfigSchemaDir string
	GlobalFile      string
	ThemeFile       string
	InfraFile       string
	GlobalSchema    string
	ThemeSchema     string
	InfraSchema     string

	AssetsDir      string
	FontsDir       string
	ImgDir         string
	IconsDir       string
	IdentitiesFile string

	SiteDir        string
	SiteSrc        string
	GeneratedDir   string
	GeneratedIcons string
	PublicDir      string
	PublicAssets   string
	DistDir        string

	InfraDir     string
	InfraSrcDir  string
	InfraDistDir string
	SrcDir       string // compatibility alias for InfraSrcDir
}

// Resolve builds a Paths set from an optional explicit root or start directory.
// When rootOrStart is empty, discovery walks upward from the current directory.
func Resolve(rootOrStart string) (*Paths, error) {
	root, err := resolveRoot(rootOrStart)
	if err != nil {
		return nil, err
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	dataDir := filepath.Join(root, "config", "data")
	schemaDir := filepath.Join(root, "config", "schema")
	infraSrc := filepath.Join(root, "infra", "src")
	infraDist := filepath.Join(root, "infra", "dist")
	return &Paths{
		Root:            root,
		ConfigDir:       filepath.Join(root, "config"),
		ConfigDataDir:   dataDir,
		ConfigSchemaDir: schemaDir,
		GlobalFile:      filepath.Join(dataDir, "global.yaml"),
		ThemeFile:       filepath.Join(dataDir, "theme.yaml"),
		InfraFile:       filepath.Join(dataDir, "infra.yaml"),
		GlobalSchema:    filepath.Join(schemaDir, "global.schema.json"),
		ThemeSchema:     filepath.Join(schemaDir, "theme.schema.json"),
		InfraSchema:     filepath.Join(schemaDir, "infra.schema.json"),

		AssetsDir:      filepath.Join(root, "assets"),
		FontsDir:       filepath.Join(root, "assets", "fonts"),
		ImgDir:         filepath.Join(root, "assets", "img"),
		IconsDir:       filepath.Join(root, "assets", "icons"),
		IdentitiesFile: filepath.Join(root, "assets", "design", "overplane-identities-50.html"),

		SiteDir:        filepath.Join(root, "site"),
		SiteSrc:        filepath.Join(root, "site", "src"),
		GeneratedDir:   filepath.Join(root, "site", "src", "generated"),
		GeneratedIcons: filepath.Join(root, "site", "src", "generated", "icons"),
		PublicDir:      filepath.Join(root, "site", "public"),
		PublicAssets:   filepath.Join(root, "site", "public", "assets"),
		DistDir:        filepath.Join(root, "site", "dist"),

		InfraDir:     filepath.Join(root, "infra"),
		InfraSrcDir:  infraSrc,
		InfraDistDir: infraDist,
		SrcDir:       infraSrc,
	}, nil
}

func resolveRoot(rootOrStart string) (string, error) {
	if rootOrStart == "" {
		wd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		return discoverRoot(wd)
	}
	abs, err := filepath.Abs(rootOrStart)
	if err != nil {
		return "", err
	}
	if isRepoRoot(abs) {
		return abs, nil
	}
	return discoverRoot(abs)
}

func discoverRoot(start string) (string, error) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	for {
		if isRepoRoot(dir) {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf(
				"could not locate repository root (no %s found in any parent of %s); pass --root",
				dataMarkerFile,
				start,
			)
		}
		dir = parent
	}
}

func isRepoRoot(dir string) bool {
	for _, marker := range []string{dataMarkerFile, schemaMarkerFile} {
		st, err := os.Stat(filepath.Join(dir, marker))
		if err != nil || st.IsDir() {
			return false
		}
	}
	return true
}
