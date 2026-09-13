package input

type Kind string

const (
	KindTick         Kind = "tick"
	KindValue        Kind = "value"
	KindIncrement    Kind = "increment"
	KindSetTotal     Kind = "set_total"
	KindPhase        Kind = "phase"
	KindLabel        Kind = "label"
	KindMeta         Kind = "meta"
	KindReset        Kind = "reset"
	KindOnComplete   Kind = "on_complete"
	KindSetSubphases Kind = "set_subphases"
	KindSubphase     Kind = "subphase"
)

type Event struct {
	Kind       Kind
	Amount     float64
	Value      float64
	Total      int
	PhaseIndex int
	PhaseName  string
	Label      string
	Meta       map[string]string
	Phases     []string
	Command    string
	// parent phase is optional: an empty name targets the current phase.
	ParentPhase    string
	SubphaseName   string
	SubphaseIndex  int
	Subphases      []string
	PhaseSubphases map[int][]string
}
