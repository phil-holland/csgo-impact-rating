package internal

const (
	// Version denotes the current application version (following semantic
	// versioning)
	Version string = "0.5.1"

	// TickRoundStart denotes the tick at the very start of the round
	// (after freezetime)
	TickRoundStart string = "roundStart"

	// TickPreDamage denotes the tick where damage is being done, but before
	// the damage has been processed
	TickPreDamage string = "prePlayerDamaged"

	// TickDamage denotes the tick where damage is being done, after the
	// damage has been processed
	TickDamage string = "playerDamaged"

	// TickBombPlant denotes the tick where the bomb has just been planted
	TickBombPlant string = "bombPlanted"

	// TickPreBombDefuse denotes the tick where the bomb has been defused, but
	// before the bomb defusal has been processed
	TickPreBombDefuse string = "preBombDefused"

	// TickBombDefuse denotes the tick where the bomb has been defused,
	// after the bomb defusal has been processed
	TickBombDefuse string = "bombDefused"

	// TickBombExplode denotes the tick where the bomb has exploded
	TickBombExplode string = "bombExploded"

	// TickItemPickedUp denotes the tick where an item has been picked up (or
	// bought) by a player
	TickItemPickedUp string = "itemPickedUp"

	// TickItemDrop denotes the tick where an item has been dropped (or used)
	// by a player
	TickItemDrop string = "itemDropped"

	// ActionDamage represents a player damaging another player
	ActionDamage string = "damage"

	// ActionTradeDamage represents a hurt player's attacker being damaged
	ActionTradeDamage string = "tradeDamage"

	// ActionFlashAssist represents a player flashing another player getting
	// damaged
	ActionFlashAssist string = "flashAssist"

	// ActionHurt represents a player being damaged
	ActionHurt string = "hurt"

	// ActionDefuse represents a player defusing the bomb
	ActionDefuse string = "defuse"

	// ActionDefusedOn represents a T player being alive when the bomb gets
	// defused
	ActionDefusedOn string = "defusedOn"
)
