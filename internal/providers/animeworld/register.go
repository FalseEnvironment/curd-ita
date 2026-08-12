package animeworld

import "github.com/wraient/curd/internal/providers"

func init() {
	providers.Register(providers.Meta{
		Name:     "animeworld",
		Aliases:  []string{"anime-world", "anime world", "aw", "animeworld.ac"},
		Referrer: baseURL + "/",
	}, func() providers.Provider {
		return &Provider{}
	})
}
