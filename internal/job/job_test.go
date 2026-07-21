package job

import "testing"

func makeJob() *Job {
	return New("https://example.com/video", ModeVideo, 1080, "original", "/tmp", "%(title)s")
}

func TestNewJobStartsInFetchingState(t *testing.T) {
	if got := makeJob().State; got != StateFetching {
		t.Errorf("State = %q, want %q", got, StateFetching)
	}
}

func TestEachJobGetsAUniqueID(t *testing.T) {
	if makeJob().ID == makeJob().ID {
		t.Error("two jobs got the same ID")
	}
}

func allStates() []State {
	states := make([]State, 0, len(validTransitions))
	for s := range validTransitions {
		states = append(states, s)
	}
	return states
}

func TestValidTransitionsSucceed(t *testing.T) {
	for from, tos := range validTransitions {
		for to := range tos {
			t.Run(string(from)+"->"+string(to), func(t *testing.T) {
				j := makeJob()
				j.State = from
				j.SetState(to)
				if j.State != to {
					t.Errorf("State = %q, want %q", j.State, to)
				}
			})
		}
	}
}

func TestInvalidTransitionsPanic(t *testing.T) {
	for _, from := range allStates() {
		allowed := validTransitions[from]
		for _, to := range allStates() {
			if allowed[to] {
				continue
			}
			from, to := from, to
			t.Run(string(from)+"->"+string(to), func(t *testing.T) {
				defer func() {
					if r := recover(); r == nil {
						t.Errorf("SetState(%q) from %q did not panic", to, from)
					}
				}()
				j := makeJob()
				j.State = from
				j.SetState(to)
			})
		}
	}
}

func TestTerminalStatesAllowNoFurtherTransitions(t *testing.T) {
	for state := range TerminalStates {
		if len(validTransitions[state]) != 0 {
			t.Errorf("terminal state %q has outgoing transitions: %v", state, validTransitions[state])
		}
	}
}

func TestActiveAndTerminalStatesPartitionAllStates(t *testing.T) {
	for s := range ActiveStates {
		if TerminalStates[s] {
			t.Errorf("state %q is in both ActiveStates and TerminalStates", s)
		}
	}
	for _, s := range allStates() {
		if !ActiveStates[s] && !TerminalStates[s] {
			t.Errorf("state %q is in neither ActiveStates nor TerminalStates", s)
		}
	}
}
