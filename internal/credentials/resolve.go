package credentials

const (
	EnvConfigDir      = "SL_CONFIG_DIR"
	EnvProfile        = "SL_PROFILE"
	EnvAPIKey         = "SL_API_KEY"
	EnvOrganizationID = "SL_ORGANIZATION_ID"
	EnvNamespaceID    = "SL_NAMESPACE_ID"
	EnvAPIDomain      = "SL_API_DOMAIN"
	DefaultAPIDomain  = "saleslumenapis.com"
)

type Env struct {
	ConfigDir      string
	Profile        string
	APIKey         string
	OrganizationID string
	NamespaceID    string
	APIDomain      string
}

type Flags struct {
	Profile        string
	OrganizationID string
	NamespaceID    string
}

type Resolved struct {
	Profile        string
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

func EnvFrom(getenv func(string) string) Env {
	return Env{
		ConfigDir:      getenv(EnvConfigDir),
		Profile:        getenv(EnvProfile),
		APIKey:         getenv(EnvAPIKey),
		OrganizationID: getenv(EnvOrganizationID),
		NamespaceID:    getenv(EnvNamespaceID),
		APIDomain:      getenv(EnvAPIDomain),
	}
}

func Resolve(flags Flags, env Env, file *File) (Resolved, error) {
	resolved := ResolveProfile(flags, env, file)
	if !resolved.HasCredentials() {
		return Resolved{}, ErrNoCredentials
	}
	return resolved, nil
}

func ResolveProfile(flags Flags, env Env, file *File) Resolved {
	if file == nil {
		file = newFile()
	}
	profile := firstNonEmpty(flags.Profile, env.Profile, DefaultProfile)
	fields := file.Fields(profile)
	resolved := Resolved{
		Profile:        profile,
		OrganizationID: firstNonEmpty(flags.OrganizationID, env.OrganizationID, fields.OrganizationID),
		NamespaceID:    firstNonEmpty(flags.NamespaceID, env.NamespaceID, fields.NamespaceID),
		APIDomain:      firstNonEmpty(env.APIDomain, fields.APIDomain, DefaultAPIDomain),
	}
	mode := profileAuthMode(fields)
	if mode == AuthModeOAuth {
		resolved.AuthMode = AuthModeOAuth
		resolved.AccessToken = fields.AccessToken
		resolved.RefreshToken = fields.RefreshToken
		resolved.OAuthExpiresAt = fields.OAuthExpiresAt
		resolved.OAuthUserID = fields.OAuthUserID
		return resolved
	}
	apiKey := firstNonEmpty(env.APIKey, fields.APIKey)
	if apiKey != "" || mode == AuthModeAPIKey {
		resolved.AuthMode = AuthModeAPIKey
		resolved.APIKey = apiKey
	}
	return resolved
}

func (r Resolved) HasCredentials() bool {
	switch r.AuthMode {
	case AuthModeOAuth:
		return r.AccessToken != "" || r.RefreshToken != ""
	case AuthModeAPIKey:
		return r.APIKey != ""
	default:
		return false
	}
}

func profileAuthMode(fields Profile) string {
	switch fields.AuthMode {
	case AuthModeOAuth:
		return AuthModeOAuth
	case AuthModeAPIKey:
		return AuthModeAPIKey
	}
	if fields.AccessToken != "" || fields.RefreshToken != "" {
		return AuthModeOAuth
	}
	if fields.APIKey != "" {
		return AuthModeAPIKey
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
