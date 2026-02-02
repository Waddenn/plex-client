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
