package router

// Stoppable is implemented by background workers that support graceful
// shutdown. router.Setup returns a []Stoppable that main.go calls Stop()
// on in reverse startup order when a SIGTERM/SIGINT is received.
type Stoppable interface {
	Stop()
}
