package internal

// TickTypeRoundStart denotes the tick at the very start of the round
// (after freezetime)
const TickTypeRoundStart = "roundStart"

// TickTypePreDamage denotes the tick where damage is being done, but before
// the damage has been processed
const TickTypePreDamage = "prePlayerDamaged"

// TickTypeDamage denotes the tick where damage is being done, after the
// damage has been processed
const TickTypeDamage = "playerDamaged"

// TickTypeBombPlant denotes the tick where the bomb has just been planted
const TickTypeBombPlant = "bombPlanted"

// TickTypePreBombDefuse denotes the tick where the bomb has been defused, but
// before the bomb defusal has been processed
const TickTypePreBombDefuse = "preBombDefused"

// TickTypeBombDefuse denotes the tick where the bomb has been defused,
// after the bomb defusal has been processed
const TickTypeBombDefuse = "bombDefused"

// TickTypeItemPickup denotes the tick where an item has been picked up (or
// bought) by a player
const TickTypeItemPickup = "itemPickup"

// TickTypeItemDrop denotes the tick where an item has been dropped (or used)
// by a player
const TickTypeItemDrop = "itemDrop"

// ActionDamage represents a player damaging another player
const ActionDamage string = "damage"

// ActionTradeDamage represents a hurt player's attacker being damaged
const ActionTradeDamage string = "tradeDamage"

// ActionFlashAssist represents a player flashing another player getting damaged
const ActionFlashAssist string = "flashAssist"

// ActionHurt represents a player being damaged
const ActionHurt string = "hurt"

// ActionDefuse represents a player defusing the bomb
const ActionDefuse string = "defuse"

// ActionDefusedOn represents a T player being alive when the bomb gets defused
const ActionDefusedOn string = "defusedOn"

// TaggedDemo holds all the data required in a tagged demo json file - the
// outermost element
type TaggedDemo struct {
	Ticks []Tick `json:"ticks"`
}

// Tick holds data related to a single in-game tick
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

// Team holds data describing a team within the game
type Team struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// Player holds data describing a player within the game
type Player struct {
	SteamID uint64 `json:"steamID"`
	Name    string `json:"name"`
	TeamID  int    `json:"teamID"`
}

// GameState holds the specific round state information used for model
// training/inference
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

// Tag holds data for a single tick tag
type Tag struct {
	Action string `json:"action"`
	Player uint64 `json:"player"`
}

// Rating holds all the data required in a rating demo json file - the
// outermost element
type Rating struct {
	RoundsPlayed            int                      `json:"roundsPlayed"`
	Players                 []PlayerRating           `json:"players"`
	RatingChanges           []RatingChange           `json:"ratingChanges"`
	RoundOutcomePredictions []RoundOutcomePrediction `json:"roundOutcomePredictions"`
}

// PlayerRating holds rating summary data for a single player
type PlayerRating struct {
	SteamID       uint64        `json:"steamID"`
	Name          string        `json:"name"`
	OverallRating OverallRating `json:"overallRating"`
	RoundRatings  []RoundRating `json:"roundRatings"`
}

// RatingChange holds data describing an individual rating change
type RatingChange struct {
	Tick   int     `json:"tick"`
	Round  int     `json:"round"`
	Player uint64  `json:"player"`
	Change float64 `json:"change"`
	Action string  `json:"action"`
}

// RoundOutcomePrediction holds data describing the round outcome prediction
// at a specific tick
type RoundOutcomePrediction struct {
	Tick              int     `json:"tick"`
	Round             int     `json:"round"`
	OutcomePrediction float64 `json:"outcomePrediction"`
}

// OverallRating holds overall rating summary data for a single player
type OverallRating struct {
	AverageRating   float64         `json:"averageRating"`
	RatingBreakdown RatingBreakdown `json:"ratingBreakdown"`
}

// RoundRating holds single-round rating summary data for a single player
type RoundRating struct {
	Round           int             `json:"round"`
	TotalRating     float64         `json:"totalRating"`
	RatingBreakdown RatingBreakdown `json:"ratingBreakdown"`
}

// RatingBreakdown holds data to describe how an overall/single round rating is
// broken down into constituent actions
type RatingBreakdown struct {
	DamageRating      float64 `json:"damageRating"`
	FlashAssistRating float64 `json:"flashAssistRating"`
	TradeDamageRating float64 `json:"tradeDamageRating"`
	DefuseRating      float64 `json:"defuseRating"`
	HurtRating        float64 `json:"hurtRating"`
}
