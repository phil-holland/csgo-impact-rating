package internal

const TickTypeRoundStart = "roundStart"
const TickTypePreDamage = "prePlayerDamaged"
const TickTypeDamage = "playerDamaged"
const TickTypeBombPlant = "bombPlanted"
const TickTypePreBombDefuse = "preBombDefused"
const TickTypeBombDefuse = "bombDefused"
const TickTypeItemPickup = "itemPickup"
const TickTypeItemDrop = "itemDrop"

// ActionDamage = player damaging another player
const ActionDamage string = "damage"

// ActionTradeDamage = killed player's killer being damaged
const ActionTradeDamage string = "tradeDamage"

// ActionFlashAssist = player flashed another player getting damaged
const ActionFlashAssist string = "flashAssist"

// ActionHurt = player being damaged
const ActionHurt string = "hurt"

// ActionDefuse = player has defused the bomb
const ActionDefuse string = "defuse"

// ActionDefusedOn = T player is alive when the bomb gets defused
const ActionDefusedOn string = "defusedOn"

type Demo struct {
	Ticks []Tick `json:"ticks"`
}

type Tick struct {
	Tick        int       `json:"tick"`
	Type        string    `json:"type"`
	ScoreCT     int       `json:"scoreCT"`
	ScoreT      int       `json:"scoreT"`
	TeamCT      Team      `json:"teamCt"`
	TeamT       Team      `json:"teamT"`
	Players     []Player  `json:"players"`
	GameState   GameState `json:"gameState"`
	Tags        []Tag     `json:"tags"`
	RoundWinner uint      `json:"roundWinner"`
}

type Team struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type Player struct {
	SteamID uint64 `json:"steamID"`
	Name    string `json:"name"`
	TeamID  int    `json:"teamID"`
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
	BombDefused  bool    `json:"bombDefused"`
}

type Tag struct {
	Action string `json:"action"`
	Player uint64 `json:"player"`
}

type RatingBreakdown struct {
	DamageRating      float64 `json:"damageRating"`
	FlashAssistRating float64 `json:"flashAssistRating"`
	TradeDamageRating float64 `json:"tradeDamageRating"`
	DefuseRating      float64 `json:"defuseRating"`
	HurtRating        float64 `json:"hurtRating"`
}

type OverallRating struct {
	AverageRating   float64         `json:"averageRating"`
	RatingBreakdown RatingBreakdown `json:"ratingBreakdown"`
}

type RoundRating struct {
	Round           int             `json:"round"`
	TotalRating     float64         `json:"totalRating"`
	RatingBreakdown RatingBreakdown `json:"ratingBreakdown"`
}

type PlayerRating struct {
	SteamID       uint64        `json:"steamID"`
	Name          string        `json:"name"`
	OverallRating OverallRating `json:"overallRating"`
	RoundRatings  []RoundRating `json:"roundRatings"`
}

type RatingChange struct {
	Tick   int     `json:"tick"`
	Round  int     `json:"round"`
	Player uint64  `json:"player"`
	Change float64 `json:"change"`
	Action string  `json:"action"`
}

type RoundOutcomePrediction struct {
	Tick              int     `json:"tick"`
	Round             int     `json:"round"`
	OutcomePrediction float64 `json:"outcomePrediction"`
}

type Rating struct {
	RoundsPlayed            int                      `json:"roundsPlayed"`
	Players                 []PlayerRating           `json:"players"`
	RatingChanges           []RatingChange           `json:"ratingChanges"`
	RoundOutcomePredictions []RoundOutcomePrediction `json:"roundOutcomePredictions"`
}

type RoundTimes struct {
	StartTick  int
	PlantTick  int
	DefuseTick int
}
