package plex

import (
	"encoding/xml"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"time"
)

type Client struct {
	BaseURL           string
	Token             string
	MachineIdentifier string
	Headers           map[string]string
	Client            *http.Client
}

func New(baseURL, token, clientIdentifier string) *Client {
	return &Client{
		BaseURL: baseURL,
		Token:   token,
		Headers: map[string]string{
			"X-Plex-Client-Identifier": clientIdentifier,
			"X-Plex-Product":           "Plex Client Go",
			"X-Plex-Version":           "0.1.0",
			"X-Plex-Device":            "Linux",
			"X-Plex-Platform":          "Linux",
			"Accept":                   "application/xml",
		},
		Client: &http.Client{Timeout: 10 * time.Second},
	}
}

type MediaContainer struct {
	MachineIdentifier string      `xml:"machineIdentifier,attr"`
	Directories       []Directory `xml:"Directory"`
	Videos            []Video     `xml:"Video"`
}

type Directory struct {
	RatingKey string `xml:"ratingKey,attr"`
	Key       string `xml:"key,attr"`
	Title     string `xml:"title,attr"`
	Type      string `xml:"type,attr"`
	Index     string `xml:"index,attr"` // Season index
	Summary   string `xml:"summary,attr"`
	Year      int     `xml:"year,attr"`
	Rating    float64 `xml:"rating,attr"`
	Genre     []Tag   `xml:"Genre"`
	UpdatedAt int64   `xml:"updatedAt,attr"`
}

type Video struct {
	RatingKey            string  `xml:"ratingKey,attr"`
	Key                  string  `xml:"key,attr"`
	ParentRatingKey      string  `xml:"parentRatingKey,attr"`
	GrandparentRatingKey string  `xml:"grandparentRatingKey,attr"`
	Title                string  `xml:"title,attr"`
	Summary              string  `xml:"summary,attr"`
	Year                 int     `xml:"year,attr"`
	Index                int     `xml:"index,attr"` // Episode index
	ParentIndex          int     `xml:"parentIndex,attr"` // Season index
	Duration             int     `xml:"duration,attr"`
	Rating               float64 `xml:"rating,attr"`
	OriginallyAvailableAt string `xml:"originallyAvailableAt,attr"`
	Type                 string  `xml:"type,attr"`
	GrandparentTitle     string  `xml:"grandparentTitle,attr"`
	ViewOffset           int     `xml:"viewOffset,attr"`
	Media                []Media `xml:"Media"`
	Genre                []Tag   `xml:"Genre"`
}

type Media struct {
	Part []Part `xml:"Part"`
}

type Part struct {
	Key string `xml:"key,attr"`
}

type Tag struct {
	Tag string `xml:"tag,attr"`
}

// Do sends an HTTP request with standard headers and retry logic.
func (c *Client) Do(req *http.Request) (*http.Response, error) {
	// Add standard headers
	for k, v := range c.Headers {
		req.Header.Set(k, v)
	}
	// Add token header
	if c.Token != "" {
		req.Header.Set("X-Plex-Token", c.Token)
	}

	maxRetries := 3
	var lastErr error

	for i := 0; i <= maxRetries; i++ {
		if i > 0 {
			// Exponential backoff: 0.5s, 1s, 2s
			backoff := time.Duration(math.Pow(2, float64(i-1))) * 500 * time.Millisecond
			time.Sleep(backoff)
		}

		resp, err := c.Client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		// Check for server errors (5xx)
		if resp.StatusCode >= 500 {
			resp.Body.Close()
			lastErr = fmt.Errorf("server error: %d", resp.StatusCode)
			continue
		}

		return resp, nil
	}

	return nil, fmt.Errorf("request failed after %d retries: %v", maxRetries, lastErr)
}


func (c *Client) GetSections() ([]Directory, error) {
	url := fmt.Sprintf("%s/library/sections", c.BaseURL)
	var mc MediaContainer
	if err := c.getXML(url, &mc); err != nil {
		return nil, err
	}
	return mc.Directories, nil
}

func (c *Client) GetSectionAll(key string) ([]Video, error) {
	url := fmt.Sprintf("%s/library/sections/%s/all", c.BaseURL, key)
	var mc MediaContainer
	if err := c.getXML(url, &mc); err != nil {
		return nil, err
	}
	return mc.Videos, nil
}

func (c *Client) GetOnDeck(key string) ([]Video, error) {
	url := fmt.Sprintf("%s/library/sections/%s/onDeck", c.BaseURL, key)
	var mc MediaContainer
	if err := c.getXML(url, &mc); err != nil {
		return nil, err
	}
	return mc.Videos, nil
}

func (c *Client) GetChildren(key string) ([]Directory, []Video, error) {
	url := fmt.Sprintf("%s/library/metadata/%s/children", c.BaseURL, key)
	var mc MediaContainer
	if err := c.getXML(url, &mc); err != nil {
		return nil, nil, err
	}
	return mc.Directories, mc.Videos, nil
}

func (c *Client) GetMetadata(key string) (*Video, error) {
	url := fmt.Sprintf("%s/library/metadata/%s", c.BaseURL, key)
	var mc MediaContainer
	if err := c.getXML(url, &mc); err != nil {
		return nil, err
	}
	if len(mc.Videos) > 0 {
		return &mc.Videos[0], nil
	}
	return nil, fmt.Errorf("no metadata found for key %s", key)
}

func (c *Client) GetSectionDirs(key string) ([]Directory, error) {
	url := fmt.Sprintf("%s/library/sections/%s/all", c.BaseURL, key)
	var mc MediaContainer
	if err := c.getXML(url, &mc); err != nil {
		return nil, err
	}
	return mc.Directories, nil
}

func (c *Client) getXML(url string, target interface{}) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}

	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("plex api error: %d", resp.StatusCode)
	}

	return xml.NewDecoder(resp.Body).Decode(target)
}

func (c *Client) ReportProgress(key string, timeMs int64, durationMs int64, state string) error {
	metadataPath := fmt.Sprintf("/library/metadata/%s", key)
	// Headers will inject token, so remove from URL
	url := fmt.Sprintf("%s/:/timeline?ratingKey=%s&key=%s&state=%s&time=%d&duration=%d",
		c.BaseURL, key, metadataPath, state, timeMs, durationMs)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}

	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != 200 {
		return fmt.Errorf("plex timeline error: %d", resp.StatusCode)
	}
	return nil
}

func (c *Client) Scrobble(key string) error {
	// Headers will inject token, so remove from URL
	url := fmt.Sprintf("%s/:/scrobble?key=%s&identifier=com.plexapp.plugins.library",
		c.BaseURL, key)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	
	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != 200 {
		return fmt.Errorf("plex scrobble error: %d", resp.StatusCode)
	}
	return nil
}

type PlayQueue struct {
	PlayQueueID          string `xml:"playQueueID,attr"`
	PlayQueueSelectedItemID string `xml:"playQueueSelectedItemID,attr"`
	PlayQueueSelectedItemOffset int `xml:"playQueueSelectedItemOffset,attr"`
	Items                []Video `xml:"Video"`
}

type PlayQueueContainer struct {
	PlayQueueID          string `xml:"playQueueID,attr"`
	PlayQueueSelectedItemID string `xml:"playQueueSelectedItemID,attr"`
	Items                []Video `xml:"Video"`
}


func (c *Client) GetMachineIdentifier() (string, error) {
	if c.MachineIdentifier != "" {
		return c.MachineIdentifier, nil
	}
	// Fetch root to get identifier
	var mc MediaContainer
	if err := c.getXML(c.BaseURL, &mc); err != nil {
		return "", err
	}
	c.MachineIdentifier = mc.MachineIdentifier
	return c.MachineIdentifier, nil
}

func (c *Client) CreatePlayQueue(item Video) (*PlayQueueContainer, error) {
	machineID, err := c.GetMachineIdentifier()
	if err != nil {
		return nil, fmt.Errorf("failed to get machine identifier: %w", err)
	}

	params := url.Values{}
	params.Set("type", "video")
	params.Set("continuous", "1") // Enable binge watching
	params.Set("repeat", "0")
	
	var uri string
	
	if item.Type == "episode" {
		// For episodes, we want the Season as the scope (URI) and the Episode as the start point (Key)
		// URI format: server://<MachineID>/com.plexapp.plugins.library/library/metadata/<SeasonID>
		params.Set("key", item.Key) // /library/metadata/<ID>
		uri = fmt.Sprintf("server://%s/com.plexapp.plugins.library/library/metadata/%s", machineID, item.ParentRatingKey)
	} else {
		// For movies, just use the item itself
		uri = fmt.Sprintf("server://%s/com.plexapp.plugins.library/library/metadata/%s", machineID, item.RatingKey)
		// Key param is optional for movies, but maybe good for consistency?
	}
	
	params.Set("uri", uri)

	endpoint := fmt.Sprintf("%s/playQueues?%s", c.BaseURL, params.Encode())
	fmt.Printf("[DEBUG] Creating PlayQueue at: %s\n", endpoint)
	
	req, err := http.NewRequest("POST", endpoint, nil)
	if err != nil {
		return nil, err
	}
	
	// Headers are handled by c.Do, but we might want to ensure specific ones?
	// The standard client headers seem sufficient based on debug tests using p.Do
	
	resp, err := c.Do(req)
	if err != nil {
		fmt.Printf("[DEBUG] PlayQueue Request failed: %v\n", err)
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		fmt.Printf("[DEBUG] PlayQueue HTTP Status: %d\n", resp.StatusCode)
		return nil, fmt.Errorf("plex playqueue error: %d", resp.StatusCode)
	}

	var mc PlayQueueContainer
	if err := xml.NewDecoder(resp.Body).Decode(&mc); err != nil {
		fmt.Printf("[DEBUG] PlayQueue Decode error: %v\n", err)
		return nil, err
	}
	
	fmt.Printf("[DEBUG] PlayQueue Created. Items: %d\n", len(mc.Items))
	return &mc, nil
}
