package cli

import (
	"errors"
	"io"

	"github.com/saleslumen/sl/internal/apiclient"
	"github.com/saleslumen/sl/internal/credentials"
	"github.com/saleslumen/sl/internal/output"
)

type IOStreams struct {
	In                      io.Reader
	Out, Err                io.Writer
	IsStdinTTY, IsStdoutTTY bool
}

type Factory struct {
	IO               *IOStreams
	Client           func() (*apiclient.Client, error)
	Organization     func() (string, error)
	Printer          func() *output.Printer
	Confirm          func(prompt string) error
	NewRequestID     func() string
	RequireUserOAuth func() error
}

var (
	ErrNoCredentials     = credentials.ErrNoCredentials
	ErrCancelled         = errors.New("cancelled")
	ErrUserOAuthRequired = errors.New("user OAuth login required")
)

type UsageError struct {
	Msg string
}

func (e *UsageError) Error() string {
	return e.Msg
}

var version = "dev"

func Version() string {
	return version
}
