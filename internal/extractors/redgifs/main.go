package redgifs

import (
	"fmt"
	"github.com/thoas/go-funk"
	"github.com/vegidio/umd-lib/event"
	"github.com/vegidio/umd-lib/fetch"
	"github.com/vegidio/umd-lib/internal"
	"github.com/vegidio/umd-lib/internal/model"
	"reflect"
	"regexp"
	"strings"
)

type Redgifs struct {
	Metadata         map[string]interface{}
	Callback         func(event event.Event)
	responseMetadata map[string]interface{}
}

func IsMatch(url string) bool {
	return internal.HasHost(url, "redgifs.com")
}

func (r *Redgifs) QueryMedia(url string, limit int, extensions []string) (*model.Response, error) {
	source, err := r.getSourceType(url)
	if err != nil {
		return nil, err
	}

	videos, err := r.fetchVideos(source, limit, extensions)
	if err != nil {
		return nil, err
	}

	media := videosToMedia(videos)
	if r.Callback != nil {
		r.Callback(event.OnQueryCompleted{Total: len(media)})
	}

	return &model.Response{
		Url:       url,
		Media:     media,
		Extractor: model.RedGifs,
		Metadata:  r.responseMetadata,
	}, nil
}

func (r *Redgifs) GetFetch() fetch.Fetch {
	return fetch.New(map[string]string{
		"User-Agent": "UMD",
	}, 0)
}

// Compile-time assertion to ensure the extractor implements the Extractor interface
var _ model.Extractor = (*Redgifs)(nil)

// region - Private methods

func (r *Redgifs) getSourceType(url string) (SourceType, error) {
	regexVideo := regexp.MustCompile(`/watch/([^/\n?]+)`)

	var source SourceType
	var id string

	switch {
	case regexVideo.MatchString(url):
		matches := regexVideo.FindStringSubmatch(url)
		id = matches[1]
		source = SourceVideo{Id: id}
	}

	if source == nil {
		return nil, fmt.Errorf("source type not found for URL: %s", url)
	}

	if r.Callback != nil {
		sourceType := strings.TrimPrefix(reflect.TypeOf(source).Name(), "Source")
		r.Callback(event.OnExtractorTypeFound{Type: sourceType, Name: id})
	}

	return source, nil
}

func (r *Redgifs) fetchVideos(source SourceType, limit int, extensions []string) ([]Video, error) {
	videos := make([]Video, 0)
	newVideos := make([]Video, 0)
	var err error

	token, err := r.getToken(r.Metadata)
	if err != nil {
		return nil, err
	}

	switch s := source.(type) {
	case SourceVideo:
		newVideos, err = r.fetchVideo(s, token)
	}

	if err != nil {
		return nil, err
	}

	videos = append(videos, newVideos...)

	if r.Callback != nil {
		queried := len(newVideos)
		r.Callback(event.OnMediaQueried{Amount: queried})
	}

	return videos, nil
}

func (r *Redgifs) getToken(metadata map[string]interface{}) (string, error) {
	token, exists := metadata["token"].(string)
	if !exists {
		auth, err := getToken()
		if err != nil {
			return "", err
		}

		token = auth.Token
	}

	if r.responseMetadata == nil {
		r.responseMetadata = make(map[string]interface{})
	}

	// Save the token to be reused in the future
	r.responseMetadata["token"] = token

	return token, nil
}

func (r *Redgifs) fetchVideo(source SourceVideo, token string) ([]Video, error) {
	video, err := getVideo(
		fmt.Sprintf("Bearer %s", token),
		fmt.Sprintf("https://www.redgifs.com/watch/%s", source.Id),
		source.Id,
	)

	if err != nil {
		return make([]Video, 0), err
	}

	return []Video{*video}, nil
}

// endregion

// region - Private functions

func videosToMedia(videos []Video) []model.Media {
	return funk.Map(videos, func(video Video) model.Media {
		return model.NewMedia(video.Gif.Url.Hd, map[string]interface{}{
			"source":   "watch",
			"name":     video.Gif.Username,
			"created":  video.Gif.Created.Time,
			"duration": video.Gif.Duration,
			"id":       video.Gif.Id,
		})
	}).([]model.Media)
}

// endregion
