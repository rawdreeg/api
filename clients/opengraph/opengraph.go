package opengraph

import (
	"context"

	og "github.com/otiai10/opengraph"
	xurls "mvdan.cc/xurls/v2"

	"github.com/hiconvo/api/log"
)

type LinkData struct {
	URL         string `json:"url"`
	Image       string `json:"image" datastore:",noindex"`
	Favicon     string `json:"favicon" datastore:",noindex"`
	Title       string `json:"title"`
	Site        string `json:"site"`
	Description string `json:"description" datastore:",noindex"`
}

type Client interface {
	Extract(ctx context.Context, text string) *LinkData
}

func NewClient() Client {
	return &clientImpl{}
}

type clientImpl struct{}

func (c *clientImpl) Extract(ctx context.Context, text string) *LinkData {
	url := xurls.Strict().FindString(text)
	if url == "" {
		return nil
	}

	data, err := og.FetchWithContext(ctx, url)
	if err != nil {
		log.Printf("opengraph.Extract(%s): %v", url, err)
		return nil
	}

	var image string
	if len(data.Image) > 0 {
		image = data.Image[0].URL
	}

	favicon := data.Favicon
	if favicon[:1] == "/" {
		favicon = data.URL.Scheme + "://" + data.URL.Hostname() + favicon
	}

	return &LinkData{
		Title:       data.Title,
		URL:         data.URL.String(),
		Site:        data.SiteName,
		Description: data.Description,
		Favicon:     favicon,
		Image:       image,
	}
}
