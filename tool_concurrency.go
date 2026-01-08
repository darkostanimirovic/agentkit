package agentkit

// ConcurrencyMode controls whether a tool can run in parallel.
type ConcurrencyMode string

const (
	ConcurrencyParallel ConcurrencyMode = "parallel"
	ConcurrencySerial   ConcurrencyMode = "serial"
)
