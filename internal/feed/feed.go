package feed

import (
	"encoding/xml"
	"fmt"
	"time"
)

type FeedMeta struct {
	Title       string
	Description string
	Language    string
	Link        string
}

// Episode holds the data needed to render a podcast item in the RSS feed.
type Episode struct {
	VideoID       string
	Title         string
	Description   string
	PublishedAt   time.Time
	DurationSecs  int
	FileSizeBytes int64
	MP3URL        string
}

// rss structs with iTunes namespace support

type rss struct {
	XMLName xml.Name `xml:"rss"`
	Version string   `xml:"version,attr"`
	Itunes  string   `xml:"xmlns:itunes,attr"`
	Channel channel  `xml:"channel"`
}

type channel struct {
	Title          string     `xml:"title"`
	Link           string     `xml:"link"`
	Description    string     `xml:"description"`
	Language       string     `xml:"language"`
	ItunesAuthor   itunesText `xml:"itunes:author"`
	ItunesExplicit itunesText `xml:"itunes:explicit"`
	Items          []item     `xml:"item"`
}

type item struct {
	Title             string     `xml:"title"`
	Description       string     `xml:"description"`
	PubDate           string     `xml:"pubDate"`
	GUID              guid       `xml:"guid"`
	Enclosure         enclosure  `xml:"enclosure"`
	ItunesDuration    itunesText `xml:"itunes:duration"`
	ItunesEpisodeType itunesText `xml:"itunes:episodeType"`
}

type guid struct {
	IsPermaLink string `xml:"isPermaLink,attr"`
	Value       string `xml:",chardata"`
}

type enclosure struct {
	URL    string `xml:"url,attr"`
	Length int64  `xml:"length,attr"`
	Type   string `xml:"type,attr"`
}

type itunesText struct {
	XMLName xml.Name
	Value   string `xml:",chardata"`
}

// Generate builds an RSS 2.0 + iTunes podcast XML document.
func Generate(meta FeedMeta, episodes []Episode) ([]byte, error) {
	ch := channel{
		Title:          meta.Title,
		Link:           meta.Link,
		Description:    meta.Description,
		Language:       meta.Language,
		ItunesAuthor:   itunesText{Value: "DIE LINKE"},
		ItunesExplicit: itunesText{Value: "false"},
	}

	for _, ep := range episodes {
		pubDate := ""
		if !ep.PublishedAt.IsZero() {
			pubDate = ep.PublishedAt.Format("Mon, 02 Jan 2006 15:04:05 +0000")
		}

		ch.Items = append(ch.Items, item{
			Title:       ep.Title,
			Description: ep.Description,
			PubDate:     pubDate,
			GUID: guid{
				IsPermaLink: "false",
				Value:       ep.VideoID,
			},
			Enclosure: enclosure{
				URL:    ep.MP3URL,
				Length: ep.FileSizeBytes,
				Type:   "audio/mpeg",
			},
			ItunesDuration:    itunesText{Value: formatDuration(ep.DurationSecs)},
			ItunesEpisodeType: itunesText{Value: "full"},
		})
	}

	f := rss{
		Version: "2.0",
		Itunes:  "http://www.itunes.com/dtds/podcast-1.0.dtd",
		Channel: ch,
	}

	data, err := xml.MarshalIndent(f, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal RSS: %w", err)
	}

	out := append([]byte(xml.Header), data...)
	return out, nil
}

func formatDuration(secs int) string {
	h := secs / 3600
	m := (secs % 3600) / 60
	s := secs % 60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}
