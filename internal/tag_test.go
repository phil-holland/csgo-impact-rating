package internal

import "testing"

// helper function to report a test failure on a call to HasMatchFinished
func testMatchFinished(t *testing.T, team1 int, team2 int, expected bool) {
	if HasMatchFinished(team1, team2, 15) != expected {
		t.Errorf("Got HasMatchFinished() = %v at a score of [%v:%v] (mr15), expected HasMatchFinished() = %v",
			!expected, team1, team2, expected)
	}
}

func TestHasMatchFinishedRegulation(t *testing.T) {
	for i := 0; i < 15; i++ {
		testMatchFinished(t, 16, i, true)
		testMatchFinished(t, i, 16, true)
	}

	for i := 0; i <= 15; i++ {
		for j := 0; j <= 15; j++ {
			testMatchFinished(t, i, j, false)
		}
	}
}

func TestHasMatchFinishedOvertime(t *testing.T) {
	testMatchFinished(t, 16, 15, false)
	testMatchFinished(t, 15, 16, false)

	testMatchFinished(t, 17, 15, false)
	testMatchFinished(t, 15, 17, false)

	testMatchFinished(t, 18, 15, false)
	testMatchFinished(t, 15, 18, false)

	testMatchFinished(t, 19, 15, true)
	testMatchFinished(t, 15, 19, true)

	testMatchFinished(t, 16, 16, false)
	testMatchFinished(t, 16, 16, false)

	testMatchFinished(t, 17, 16, false)
	testMatchFinished(t, 16, 17, false)

	testMatchFinished(t, 18, 16, false)
	testMatchFinished(t, 16, 18, false)

	testMatchFinished(t, 19, 16, true)
	testMatchFinished(t, 16, 19, true)

	testMatchFinished(t, 19, 18, false)
	testMatchFinished(t, 18, 19, false)
}
