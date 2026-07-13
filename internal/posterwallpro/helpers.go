// Package posterwallpro implements the intelligent Poster Wall Pro module.
// Helper utilities.
package posterwallpro

import (
	"fmt"
	"strings"
)

func sanitizeID(title string, year int) string {
	s := strings.ToLower(strings.TrimSpace(title))
	s = strings.Map(func(r rune) rune {
		if r == ' ' || r == '.' {
			return '_'
		}
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			return r
		}
		return -1
	}, s)
	return fmt.Sprintf("%s_%d", s, year)
}

var genreKeywords = map[string][]string{
	"Action":  {"fight", "war", "battle", "gun", "mission"},
	"Drama":   {"love", "life", "heart", "story", "family"},
	"Comedy":  {"funny", "laugh", "comedy", "happy"},
	"Horror":  {"terror", "scary", "blood", "ghost", "demon"},
	"Sci-Fi":  {"space", "alien", "future", "robot", "cyber"},
	"Fantasy": {"magic", "dragon", "wizard", "kingdom"},
	"Crime":   {"crime", "police", "detective", "gangster"},
	"Thriller": {"suspense", "mystery", "psycho"},
}

func guessGenres(title string) []string {
	t := strings.ToLower(title)
	for genre, keywords := range genreKeywords {
		for _, kw := range keywords {
			if strings.Contains(t, kw) {
				return []string{genre}
			}
		}
	}
	return []string{"Drama"} // default fallback
}

func firstGenre(p PosterEntry) string {
	if len(p.Genres) > 0 {
		return p.Genres[0]
	}
	return ""
}