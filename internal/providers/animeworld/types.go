package animeworld

// SearchItem is stored in SelectionOption.ExtraData for mapping hints.
//
// AnimeWorld's search API returns the AniList and MyAnimeList IDs for every
// entry, so provider mapping can match exactly instead of guessing by title.
type SearchItem struct {
	ID        int
	Name      string
	JTitle    string
	Episodes  int
	Dub       bool
	Year      string
	Studio    string
	MalID     int
	AnilistID int
}
