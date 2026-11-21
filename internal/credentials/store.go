package credentials

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
)

const (
	FileName             = "credentials"
	DefaultDirName       = ".sl"
	DefaultProfile       = "default"
	AuthModeOAuth        = "oauth"
	AuthModeAPIKey       = "api_key"
	KeyAPIKey            = "api_key"
	KeyAuthMode          = "auth_mode"
	KeyOAuthAccessToken  = "oauth_access_token"
	KeyOAuthRefreshToken = "oauth_refresh_token"
	KeyOAuthExpiresAt    = "oauth_expires_at"
	KeyOAuthUserID       = "oauth_user_id"
	KeyOrganizationID    = "organization_id"
	KeyNamespaceID       = "namespace_id"
	KeyAPIDomain         = "api_domain"
)

type Store struct {
	Dir string
}

type Profile struct {
	AuthMode       string
	APIKey         string
	AccessToken    string
	RefreshToken   string
	OAuthExpiresAt string
	OAuthUserID    string
	OrganizationID string
	NamespaceID    string
	APIDomain      string
}

type File struct {
	profiles map[string]map[string]any
	extras   map[string]any
}

func Dir(env Env, userHome func() (string, error)) (string, error) {
	if env.ConfigDir != "" {
		return env.ConfigDir, nil
	}
	if userHome == nil {
		userHome = os.UserHomeDir
	}
	home, err := userHome()
	if err != nil {
		return "", fmt.Errorf("credentials: home directory: %w", err)
	}
	if home == "" {
		return "", fmt.Errorf("credentials: home directory is empty")
	}
	return filepath.Join(home, DefaultDirName), nil
}

func NewStore(env Env, userHome func() (string, error)) (Store, error) {
	dir, err := Dir(env, userHome)
	if err != nil {
		return Store{}, err
	}
	return Store{Dir: dir}, nil
}

func Path(dir string) string {
	return filepath.Join(dir, FileName)
}

func (s Store) Path() string {
	return Path(s.Dir)
}

func (s Store) Load() (*File, error) {
	return Load(s.Path())
}

func (s Store) Save(file *File) error {
	return Save(s.Path(), file)
}

func (s Store) Resolve(flags Flags, env Env) (Resolved, error) {
	file, err := s.Load()
	if err != nil {
		return Resolved{}, err
	}
	return Resolve(flags, env, file)
}

func Load(path string) (*File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return newFile(), nil
		}
		return nil, fmt.Errorf("credentials: read: %w", err)
	}
	return parse(data)
}

func Save(path string, file *File) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("credentials: create dir: %w", err)
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return fmt.Errorf("credentials: chmod dir: %w", err)
	}
	data, err := file.marshal()
	if err != nil {
		return fmt.Errorf("credentials: encode: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".credentials-*.tmp")
	if err != nil {
		return fmt.Errorf("credentials: create temp: %w", err)
	}
	tmpName := tmp.Name()
	if err := writeTemp(tmp, data); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("credentials: rename: %w", err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return fmt.Errorf("credentials: chmod file: %w", err)
	}
	return nil
}

func writeTemp(tmp *os.File, data []byte) error {
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("credentials: chmod temp: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("credentials: write temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("credentials: close temp: %w", err)
	}
	return nil
}

func newFile() *File {
	return &File{profiles: map[string]map[string]any{}, extras: map[string]any{}}
}

func parse(data []byte) (*File, error) {
	raw := map[string]any{}
	if err := toml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("credentials: parse: %w", err)
	}
	file := newFile()
	for name, value := range raw {
		table, ok := asTable(value)
		if !ok {
			file.extras[name] = value
			continue
		}
		file.profiles[name] = table
	}
	return file, nil
}

func asTable(value any) (map[string]any, bool) {
	table, ok := value.(map[string]any)
	return table, ok
}

func (f *File) marshal() ([]byte, error) {
	raw := map[string]any{}
	for name, value := range f.extras {
		raw[name] = value
	}
	for name, table := range f.profiles {
		raw[name] = table
	}
	return toml.Marshal(raw)
}

func (f *File) Has(profile string) bool {
	_, ok := f.profiles[profile]
	return ok
}

func (f *File) Fields(profile string) Profile {
	table, ok := f.profiles[profile]
	if !ok {
		return Profile{}
	}
	return Profile{
		AuthMode:       stringField(table, KeyAuthMode),
		APIKey:         stringField(table, KeyAPIKey),
		AccessToken:    stringField(table, KeyOAuthAccessToken),
		RefreshToken:   stringField(table, KeyOAuthRefreshToken),
		OAuthExpiresAt: stringField(table, KeyOAuthExpiresAt),
		OAuthUserID:    stringField(table, KeyOAuthUserID),
		OrganizationID: stringField(table, KeyOrganizationID),
		NamespaceID:    stringField(table, KeyNamespaceID),
		APIDomain:      stringField(table, KeyAPIDomain),
	}
}

func (f *File) Extra(profile, key string) (any, bool) {
	table, ok := f.profiles[profile]
	if !ok {
		return nil, false
	}
	value, ok := table[key]
	return value, ok
}

func (f *File) ExtraRoot(key string) (any, bool) {
	value, ok := f.extras[key]
	return value, ok
}

func (f *File) Set(profile, key, value string) {
	if f.profiles == nil {
		f.profiles = map[string]map[string]any{}
	}
	table, ok := f.profiles[profile]
	if !ok {
		table = map[string]any{}
		f.profiles[profile] = table
	}
	table[key] = value
}

func (f *File) Unset(profile, key string) {
	table, ok := f.profiles[profile]
	if !ok {
		return
	}
	delete(table, key)
}

func (f *File) RemoveProfile(profile string) {
	delete(f.profiles, profile)
}

func (f *File) SetOAuth(profile, accessToken, refreshToken, expiresAt, userID, organizationID string) {
	f.Set(profile, KeyAuthMode, AuthModeOAuth)
	f.Set(profile, KeyOAuthAccessToken, accessToken)
	if refreshToken != "" {
		f.Set(profile, KeyOAuthRefreshToken, refreshToken)
	} else {
		f.Unset(profile, KeyOAuthRefreshToken)
	}
	f.Set(profile, KeyOAuthExpiresAt, expiresAt)
	f.Set(profile, KeyOAuthUserID, userID)
	if organizationID != "" {
		f.Set(profile, KeyOrganizationID, organizationID)
	}
	f.Unset(profile, KeyAPIKey)
}

func (f *File) SetAPIKey(profile, apiKey, organizationID string) {
	f.Set(profile, KeyAuthMode, AuthModeAPIKey)
	f.Set(profile, KeyAPIKey, apiKey)
	if organizationID != "" {
		f.Set(profile, KeyOrganizationID, organizationID)
	}
	f.Unset(profile, KeyOAuthAccessToken)
	f.Unset(profile, KeyOAuthRefreshToken)
	f.Unset(profile, KeyOAuthExpiresAt)
	f.Unset(profile, KeyOAuthUserID)
}

func stringField(table map[string]any, key string) string {
	value, ok := table[key]
	if !ok || value == nil {
		return ""
	}
	text, ok := value.(string)
	if ok {
		return text
	}
	return fmt.Sprint(value)
}
