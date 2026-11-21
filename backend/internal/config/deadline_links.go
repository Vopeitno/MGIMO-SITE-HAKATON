package config

import (
	"net/url"
	"strings"
)

const DefaultMoodleURL = "https://ed.mgimo.ru/"

func ValidSubmissionURL(link string) bool {
	u, err := url.Parse(link)
	return err == nil && len(link) <= 2048 && strings.TrimSpace(link) == link && u.Scheme == "https" && u.Hostname() != "" && u.User == nil && !strings.ContainsAny(link, "\r\n\t")
}
