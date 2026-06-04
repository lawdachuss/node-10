package router

import "testing"

func TestBuildHostPlayersGeneratesEmbeds(t *testing.T) {
	players := buildHostPlayers(map[string]string{
		"GoFile": "https://gofile.io/d/example",
		"VOE.sx": "https://voe.sx/abc123",
	})

	byHost := map[string]hostPlayer{}
	for _, player := range players {
		byHost[player.Host] = player
	}

	if got := byHost["GoFile"].EmbedURL; got != "" {
		t.Fatalf("GoFile embed URL = %q", got)
	}
	if got := byHost["GoFile"].VideoURL; got != "" {
		t.Fatalf("GoFile video URL = %q", got)
	}
	if got := byHost["VOE.sx"].EmbedURL; got != "https://voe.sx/e/abc123" {
		t.Fatalf("VOE embed URL = %q", got)
	}
}
