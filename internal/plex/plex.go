package plex

import (
	"encoding/xml"
	"fmt"
	"net/http"
	"time"
)

type Client struct {
	BaseURL string
	Token   string
	Client  *http.Client
}

func New(baseURL, token string) *Client {
	return &Client{
		BaseURL: baseURL,
		Token:   token,
		Client:  &http.Client{Timeout: 10 * time.Second},
	}
}

type MediaContainer struct {
	Directories []Directory `xml:"Directory"`
	Videos      []Video     `xml:"Video"`
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
	Title                string  `xml:"title,attr"`
	Summary              string  `xml:"summary,attr"`
	Year                 int     `xml:"year,attr"`
	Index                int     `xml:"index,attr"` // Episode index
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

func (c *Client) GetSections() ([]Directory, error) {
	url := fmt.Sprintf("%s/library/sections?X-Plex-Token=%s", c.BaseURL, c.Token)
	var mc MediaContainer
	if err := c.getXML(url, &mc); err != nil {
		return nil, err
	}
	return mc.Directories, nil
}

func (c *Client) GetSectionAll(key string) ([]Video, error) {
	url := fmt.Sprintf("%s/library/sections/%s/all?X-Plex-Token=%s", c.BaseURL, key, c.Token)
	var mc MediaContainer
	if err := c.getXML(url, &mc); err != nil {
		return nil, err
	}
	return mc.Videos, nil
}

func (c *Client) GetOnDeck(key string) ([]Video, error) {
	url := fmt.Sprintf("%s/library/sections/%s/onDeck?X-Plex-Token=%s", c.BaseURL, key, c.Token)
	var mc MediaContainer
	if err := c.getXML(url, &mc); err != nil {
		return nil, err
	}
	return mc.Videos, nil
}

func (c *Client) GetChildren(key string) ([]Directory, []Video, error) {
	url := fmt.Sprintf("%s/library/metadata/%s/children?X-Plex-Token=%s", c.BaseURL, key, c.Token)
	var mc MediaContainer
	if err := c.getXML(url, &mc); err != nil {
		return nil, nil, err
	}
	return mc.Directories, mc.Videos, nil
}

func (c *Client) GetMetadata(key string) (*Video, error) {
	url := fmt.Sprintf("%s/library/metadata/%s?X-Plex-Token=%s", c.BaseURL, key, c.Token)
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
	url := fmt.Sprintf("%s/library/sections/%s/all?X-Plex-Token=%s", c.BaseURL, key, c.Token)
	var mc MediaContainer
	if err := c.getXML(url, &mc); err != nil {
		return nil, err
	}
	return mc.Directories, nil
}

func (c *Client) getXML(url string, target interface{}) error {
	resp, err := c.Client.Get(url)
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
	// API expects ratingKey in query params but typically also accepts key in path or query
	// URL construction: /:/timeline?ratingKey=X&key=/library/metadata/X&state=playing&time=0&duration=1000...
	
	// Using ratingKey for both 'ratingKey' and 'key' param (as identifier) is a common plex pattern
	// but the 'key' param usually expects the metadata path like '/library/metadata/123'
	metadataPath := fmt.Sprintf("/library/metadata/%s", key)

	url := fmt.Sprintf("%s/:/timeline?ratingKey=%s&key=%s&state=%s&time=%d&duration=%d&X-Plex-Token=%s",
		c.BaseURL, key, metadataPath, state, timeMs, durationMs, c.Token)

	// fmt.Printf("🌐 [Debug] Plex Request: %s\n", url) // Log complete URL if needed

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	
	// Plex requires these headers for timeline
	req.Header.Set("X-Plex-Client-Identifier", "plex-client-go")
	req.Header.Set("X-Plex-Product", "Plex Client Go")
	req.Header.Set("X-Plex-Version", "0.1.0")

	resp, err := c.Client.Do(req)
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
	url := fmt.Sprintf("%s/:/scrobble?key=%s&identifier=com.plexapp.plugins.library&X-Plex-Token=%s",
		c.BaseURL, key, c.Token)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	
	req.Header.Set("X-Plex-Client-Identifier", "plex-client-go")

	resp, err := c.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != 200 {
		return fmt.Errorf("plex scrobble error: %d", resp.StatusCode)
	}
	return nil
}
