package capture

// OutputCaptor - Takes output from a stream and streams it to a callback
type OutputCaptor struct {
	OutputHandler func(p []byte) error
}

// New - Creates a new captor
func New(outputHandler func(p []byte) error) *OutputCaptor {
	return &OutputCaptor{
		OutputHandler: outputHandler,
	}
}

func (o *OutputCaptor) Write(p []byte) (int, error) {
	n := len(p)
	err := o.OutputHandler(p)
	return n, err
}
