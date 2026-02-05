package auth

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"runtime"
	"time"
)

const (
	SignInURL    = "https://plex.tv/api/v2/pins"
	PollURL      = "https://plex.tv/api/v2/pins/%d"
	ResourcesURL = "https://plex.tv/api/v2/resources?includeHttps=1"
)

type PlexPin struct {
	ID        int    `json:"id"`
	Code      string `json:"code"`
	AuthToken string `json:"authToken"`
	ExpiresAt string `json:"expiresAt"`
}

type PlexResource struct {
	Name             string           `json:"name"`
	Product          string           `json:"product"`
	Provides         string           `json:"provides"`
	ClientIdentifier string           `json:"clientIdentifier"`
	Connections      []PlexConnection `json:"connections"`
	Owned            bool             `json:"owned"`
}

type PlexConnection struct {
	Protocol string `json:"protocol"`
	Address  string `json:"address"`
	Port     int    `json:"port"`
	Uri      string `json:"uri"`
	Local    bool   `json:"local"`
}

type AuthClient struct {
	ClientID string
	Product  string
	Version  string
	Platform string
	Device   string
	Client   *http.Client
}

// ... existing NewAuthClient ...

// GetResources fetches the list of available servers
func (a *AuthClient) GetResources(token string) ([]PlexResource, error) {
	req, err := http.NewRequest("GET", ResourcesURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Plex-Token", token)
	req.Header.Set("X-Plex-Client-Identifier", a.ClientID)
	req.Header.Set("X-Plex-Product", a.Product)
	req.Header.Set("X-Plex-Version", a.Version)
	req.Header.Set("X-Plex-Platform", a.Platform)
	req.Header.Set("X-Plex-Device", a.Device)

	resp, err := a.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get resources: %s", resp.Status)
	}

	var resources []PlexResource
	if err := json.NewDecoder(resp.Body).Decode(&resources); err != nil {
		return nil, err
	}

	return resources, nil
}

func NewAuthClient(clientID, product, version string) *AuthClient {
	return &AuthClient{
		ClientID: clientID,
		Product:  product,
		Version:  version,
		Platform: runtime.GOOS,
		Device:   "Plex Client CLI",
		Client:   &http.Client{Timeout: 10 * time.Second},
	}
}

// GetPin requests a new PIN from Plex
func (a *AuthClient) GetPin() (*PlexPin, string, error) {
	req, err := http.NewRequest("POST", SignInURL, nil)
	if err != nil {
		return nil, "", err
	}

	q := req.URL.Query()
	q.Add("strong", "true")
	req.URL.RawQuery = q.Encode()

	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Plex Client CLI/0.1.0")
	req.Header.Set("X-Plex-Product", a.Product)
	req.Header.Set("X-Plex-Client-Identifier", a.ClientID)
	req.Header.Set("X-Plex-Version", a.Version)
	req.Header.Set("X-Plex-Platform", a.Platform)
	req.Header.Set("X-Plex-Device", a.Device)

	resp, err := a.Client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return nil, "", fmt.Errorf("failed to create pin: %s", resp.Status)
	}

	var pin PlexPin
	if err := json.NewDecoder(resp.Body).Decode(&pin); err != nil {
		return nil, "", err
	}

	// Construct the Auth URL with proper encoding
	params := url.Values{}
	params.Set("clientID", a.ClientID)
	params.Set("code", pin.Code)
	params.Set("context[device][product]", a.Product)
	params.Set("context[device][version]", a.Version)
	params.Set("context[device][platform]", a.Platform)
	params.Set("context[device][device]", a.Device)
	params.Set("forwardUrl", "https://app.plex.tv/desktop") // Optional: where to go after

	authLink := fmt.Sprintf("https://app.plex.tv/auth#?%s", params.Encode())
	return &pin, authLink, nil
}

// CheckPin polls the API to see if the user has authorized the PIN
func (a *AuthClient) CheckPin(pinID int) (*PlexPin, error) {
	urlStr := fmt.Sprintf(PollURL, pinID)
	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		return nil, err
	}

	// No query params needed if sending headers?
	// Some docs suggest sending 'code' as query param if available, but ID is in URL.
	// Let's keep URL clean.

	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Plex Client CLI/0.1.0")
	req.Header.Set("X-Plex-Product", a.Product)
	req.Header.Set("X-Plex-Client-Identifier", a.ClientID)
	req.Header.Set("X-Plex-Version", a.Version)
	req.Header.Set("X-Plex-Platform", a.Platform)
	req.Header.Set("X-Plex-Device", a.Device)

	resp, err := a.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to check pin: %s", resp.Status)
	}

	var pin PlexPin
	if err := json.NewDecoder(resp.Body).Decode(&pin); err != nil {
		return nil, err
	}

	return &pin, nil
}
