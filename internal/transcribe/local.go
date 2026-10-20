package transcribe

import (
	"context"
	"net/http"
)

// Local is the bundled whisper-server, started on first use.
type Local struct {
	Server *Server
	Client *http.Client
}

func (l Local) Name() string { return "local:whisper-large-v3-turbo" }

func (l Local) Transcribe(ctx context.Context, r Request) (Result, error) {
	url, err := l.Server.URL(ctx)
	if err != nil {
		return Result{}, err
	}
	return Whisper{Client: l.Client, URL: url, Local: true}.Transcribe(ctx, r)
}
