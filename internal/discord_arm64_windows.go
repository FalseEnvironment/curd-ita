//go:build windows && arm64
// +build windows,arm64

package internal

func ConvertSecondsToMinutes(seconds int) int { return seconds / 60 }

func LoginClient(clientId string) error { return nil }
func LogoutClient() error { return nil }

func DiscordPresence(anime Anime, IsPaused bool, currentPosition int, totalDuration int, clientId string) error {
	return DiscordPresenceWithForce(anime, IsPaused, currentPosition, totalDuration, clientId, false)
}

func DiscordPresenceWithForce(anime Anime, IsPaused bool, currentPosition int, totalDuration int, clientId string, forceUpdate bool) error {
	return nil
}