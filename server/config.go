package server

import (
        "encoding/json"
        "fmt"
        "strings"
        "sync"

        "github.com/teacat/chaturbate-dvr/entity"
)

var Config *entity.Config
var configMu sync.RWMutex

type persistedSettings struct {
        Cookies         string `json:"cookies"`
        CfClearance     string `json:"cf_clearance,omitempty"`
        SessionID       string `json:"sessionid,omitempty"`
        Csrftoken       string `json:"csrftoken,omitempty"`
        UserAgent string `json:"user_agent"`
        StreamtapeLogin string `json:"streamtape_login,omitempty"`
	StreamtapeKey   string `json:"streamtape_key,omitempty"`
	MixdropEmail    string `json:"mixdrop_email,omitempty"`
	MixdropToken    string `json:"mixdrop_token,omitempty"`
	PixelDrainToken string `json:"pixeldrain_token,omitempty"`
}

// SaveSettings writes the runtime cookies and user-agent to Supabase.
func SaveSettings() error {
	configMu.RLock()
        s := persistedSettings{
                Cookies:         Config.Cookies,
                CfClearance:     Config.CfClearance,
                SessionID:       Config.SessionID,
                Csrftoken:       Config.Csrftoken,
                UserAgent: Config.UserAgent,
                StreamtapeLogin:  Config.StreamtapeLogin,
		StreamtapeKey:    Config.StreamtapeKey,
		MixdropEmail:     Config.MixdropEmail,
		MixdropToken:     Config.MixdropToken,
		PixelDrainToken:  Config.PixelDrainToken,
	}
	configMu.RUnlock()

	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal settings: %w", err)
	}

	if err := SaveSettingsToDB(b); err != nil {
		return fmt.Errorf("save settings to Supabase: %w", err)
	}
	return nil
}

// LoadSettings reads persisted cookies and user-agent from Supabase.
func LoadSettings() error {
	b := LoadSettingsFromDB()
	if b == nil {
		return nil
	}

	var s persistedSettings
	if err := json.Unmarshal(b, &s); err != nil {
		return fmt.Errorf("unmarshal settings: %w", err)
	}

	configMu.Lock()
        if s.Cookies != "" {
                Config.Cookies = s.Cookies
        }
        if s.CfClearance != "" {
                Config.CfClearance = s.CfClearance
        }
        if s.SessionID != "" {
                Config.SessionID = s.SessionID
        }
        if s.Csrftoken != "" {
                Config.Csrftoken = s.Csrftoken
        }
        if s.UserAgent != "" {
                Config.UserAgent = s.UserAgent
        }
        if s.StreamtapeLogin != "" {
		Config.StreamtapeLogin = s.StreamtapeLogin
	}
	if s.StreamtapeKey != "" {
		Config.StreamtapeKey = s.StreamtapeKey
	}
	if s.MixdropEmail != "" {
		Config.MixdropEmail = s.MixdropEmail
	}
	if s.MixdropToken != "" {
		Config.MixdropToken = s.MixdropToken
	}
        if s.PixelDrainToken != "" {
                Config.PixelDrainToken = s.PixelDrainToken
        }

        // Parse Config.Cookies back into individual fields if they are empty.
        if Config.Cookies != "" {
                if Config.CfClearance == "" {
                        Config.CfClearance = extractCookie(Config.Cookies, "cf_clearance")
                }
                if Config.SessionID == "" {
                        Config.SessionID = extractCookie(Config.Cookies, "sessionid")
                }
                if Config.Csrftoken == "" {
                        Config.Csrftoken = extractCookie(Config.Cookies, "csrftoken")
                }
        }
        configMu.Unlock()

        return nil
}

func extractCookie(cookieStr, name string) string {
        for _, pair := range strings.Split(cookieStr, ";") {
                parts := strings.SplitN(strings.TrimSpace(pair), "=", 2)
                if len(parts) == 2 && strings.TrimSpace(parts[0]) == name {
                        return strings.TrimSpace(parts[1])
                }
        }
        return ""
}

// UpdateUploaderCredentials updates upload service credentials (Streamtape, Mixdrop)
// and protects concurrent access with a mutex.
func UpdateUploaderCredentials(streamtapeLogin, streamtapeKey, mixdropEmail, mixdropToken, pixeldrainToken string) {
	configMu.Lock()
	if streamtapeLogin != "" {
		Config.StreamtapeLogin = streamtapeLogin
	}
	if streamtapeKey != "" {
		Config.StreamtapeKey = streamtapeKey
	}
	if mixdropEmail != "" {
		Config.MixdropEmail = mixdropEmail
	}
	if mixdropToken != "" {
		Config.MixdropToken = mixdropToken
	}
	if pixeldrainToken != "" {
		Config.PixelDrainToken = pixeldrainToken
	}
	configMu.Unlock()
}
