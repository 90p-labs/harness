package domain

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
)

var ErrInvalidPRURL = errors.New("invalid pull request url")

var prURL = regexp.MustCompile(`^https://github\.com/([^/]+)/([^/]+)/pull/([0-9]+)$`)

type PR struct {
	Owner  string
	Repo   string
	Number int
}

func ParsePRURL(url string) (PR, error) {
	m := prURL.FindStringSubmatch(url)
	if m == nil {
		return PR{}, fmt.Errorf("%w: %q", ErrInvalidPRURL, url)
	}
	number, err := strconv.Atoi(m[3])
	if err != nil {
		return PR{}, fmt.Errorf("%w: %q", ErrInvalidPRURL, url)
	}
	return PR{Owner: m[1], Repo: m[2], Number: number}, nil
}
