package domain

type Track string

const (
	TrackLight Track = "light"
	TrackFull  Track = "full"
)

func Stricter(a, b Track) Track {
	if rank(b) > rank(a) {
		return b
	}
	return a
}

func rank(t Track) int {
	switch t {
	case TrackFull:
		return 2
	case TrackLight:
		return 1
	}
	return 0
}
