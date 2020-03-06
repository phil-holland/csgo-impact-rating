package internal

const TickTypeDamage = "player_damaged"
const TickTypeBombPlant = "bomb_planted"
const TickTypeBombDefuse = "bomb_defused"

// ActionDamage = player damaging another player
const ActionDamage string = "damage"

// ActionDamageBlind = player damaging a blind player
// TODO: implement "flash assist" events
//const ActionDamageBlind string = "damageBlind"

// ActionHurt = player being damaged
const ActionHurt string = "hurt"

// ActionDefuse = player has defused the bomb
const ActionDefuse string = "defuse"

type Demo struct {
	Team1 Team `json:"team1"`
	Team2 Team `json:"team2"`

	Players []Player `json:"players"`
	Ticks   []Tick   `json:"ticks"`
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
	Type      string    `json:"type"`
	GameState GameState `json:"gameState"`
	Tags      []Tag     `json:"tags"`
}

type GameState struct {
	AliveCT      int     `json:"aliveCT"`
	AliveT       int     `json:"aliveT"`
	MeanHealthCT float64 `json:"meanHealthCT"`
	MeanHealthT  float64 `json:"meanHealthT"`
	MeanValueCT  float64 `json:"meanValueCT"`
	MeanValueT   float64 `json:"meanValueT"`
	RoundTime    float64 `json:"roundTime"`
	BombPlanted  bool    `json:"bombPlanted"`
}

type Tag struct {
	Action string `json:"action"`
	Player int64  `json:"player"`
}
