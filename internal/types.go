package internal

// TaggedDemo holds all the data required in a tagged demo json file - the
// outermost element
type TaggedDemo struct {
	TaggedDemoMetadata TaggedDemoMetadata `json:"metadata"`
	Ticks              []Tick             `json:"ticks"`
}

// TaggedDemoMetadata holds all the metadata (version etc.) for a tagged
// demo file
type TaggedDemoMetadata struct {
	Version string `json:"version"`
}

// Tick holds data related to a single in-game tick
type Tick struct {
	Tick        int       `json:"tick"`
	Type        string    `json:"type"`
	ScoreCT     int       `json:"scoreCT"`
	ScoreT      int       `json:"scoreT"`
	TeamCT      Team      `json:"teamCT"`
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
	BombTime     float64 `json:"bombTime"`
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
	RatingMetadata          RatingMetadata           `json:"metadata"`
	RoundsPlayed            int                      `json:"roundsPlayed"`
	Players                 []PlayerRating           `json:"players"`
	RatingChanges           []RatingChange           `json:"ratingChanges"`
	RoundOutcomePredictions []RoundOutcomePrediction `json:"roundOutcomePredictions"`
}

// RatingMetadata holds all the metadata (version etc.) for a rating
// demo json file
type RatingMetadata struct {
	Version string `json:"version"`
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
	Round  Round   `json:"round"`
	Player uint64  `json:"player"`
	Change float64 `json:"change"`
	Action string  `json:"action"`
}

// Round holds helper data describing a single round
type Round struct {
	Number  int `json:"number"`
	ScoreCT int `json:"scoreCT"`
	ScoreT  int `json:"scoreT"`
}

// RoundOutcomePrediction holds data describing the round outcome prediction
// at a specific tick
type RoundOutcomePrediction struct {
	Tick              int     `json:"tick"`
	Round             Round   `json:"round"`
	OutcomePrediction float64 `json:"outcomePrediction"`
}

// OverallRating holds overall rating summary data for a single player
type OverallRating struct {
	AverageRating   float64         `json:"averageRating"`
	RatingBreakdown RatingBreakdown `json:"ratingBreakdown"`
}

// RoundRating holds single-round rating summary data for a single player
type RoundRating struct {
	Round           Round           `json:"round"`
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
	AliveRating       float64 `json:"aliveRating"`
}
