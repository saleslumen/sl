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
	APIKey         string
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
	if resolved.APIKey == "" {
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
	return Resolved{
		Profile:        profile,
		APIKey:         firstNonEmpty(env.APIKey, fields.APIKey),
		OrganizationID: firstNonEmpty(flags.OrganizationID, env.OrganizationID, fields.OrganizationID),
		NamespaceID:    firstNonEmpty(flags.NamespaceID, env.NamespaceID, fields.NamespaceID),
		APIDomain:      firstNonEmpty(env.APIDomain, fields.APIDomain, DefaultAPIDomain),
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
