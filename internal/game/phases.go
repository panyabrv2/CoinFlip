package game

type Phase string

const (
	PhaseWaiting       Phase = "waiting"
	PhaseBetting       Phase = "betting"
	PhaseGettingResult Phase = "gettingResult"
	PhaseFinished      Phase = "finished"
)
