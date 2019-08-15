package player

type Metrics struct {
	ServingStreamsCount int
}

type PlayerService struct {
	Metrics *Metrics
}

type Player struct {
	reflectedStream
	URI string
}

func (ps *PlayerService) NewPlayer(uri string) *Player {
	return &Player{URI: uri}
}
