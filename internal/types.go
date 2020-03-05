package internal

type Demo struct {
	Team1 Team `json:"team1"`
	Team2 Team `json:"team2"`

	Players []Player `json:"players"`
}

type Team struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Flag  string `json:"flag"`
	Score int    `json:"score"`
}

type Player struct {
	SteamID int64  `json:"steamID"`
	Name    string `json:"name"`
	TeamID  int    `json:"teamID"`
}

type Tick struct {
	Tags []Tag `json:"tags"`
}

type Tag struct {
	Action        string `json:"action"`
	PlayerFor     int64  `json:"playerFor"`
	PlayerAgainst int64  `json:"playerAgainst"`
}
