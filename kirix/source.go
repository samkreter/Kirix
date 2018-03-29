package kirix

type Work struct {
	msg string
}

type Source interface {
	GetWork() Work
}
