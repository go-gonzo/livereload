// This package provide a Slurp `stage` from https://github.com/omeid/livereload

package livereload

import (
	"fmt"
	"html/template"
	"net/http"
	"sync/atomic"

	"github.com/omeid/gonzo"
	"github.com/omeid/gonzo/context"
	"github.com/omeid/kargar"

	"github.com/omeid/go-livereload"
)

var (
	LivereloadScript = livereload.LivereloadScript
)

const serverName = "gonzo-livereload"

var Endpoint = "http://localhost:35729/"

type Config struct {
	LiveCSS bool
}

type Server interface {
	Reload() gonzo.Stage
	Start() kargar.Action
	Client() template.HTML
}

func New(conf Config) Server {
	return &server{livecss: conf.LiveCSS}
}

type server struct {
	cb           atomic.Value
	clientScript atomic.Value
	livecss      bool
}

func (s *server) Reload() gonzo.Stage {
	return func(ctx context.Context, in <-chan gonzo.File, out chan<- gonzo.File) error {
		for {
			select {
			case file, ok := <-in:
				if !ok {
					return nil
				}
				path := file.FileInfo().Name()
				cb, ok := s.cb.Load().(func(string, bool))
				if ok {
					ctx.Infof("Reloading %s", path)
					cb(path, s.livecss)
				}
				out <- file
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
}

const clientScript = `
<script>
var lrs = document.createElement("script"); lrs.type = "text/javascript"; lrs.src = "%s";
document.body.appendChild(lrs);
</script>
`

func (s *server) Start() kargar.Action {
	return func(ctx context.Context) error {

		server := livereload.New(Endpoint)
		mux := http.NewServeMux()
		mux.Handle("/", server)
		mux.HandleFunc("/livereload.js", livereload.LivereloadScript)

		err := make(chan error)
		go func(err chan<- error) {
			err <- http.ListenAndServe(Endpoint, mux)
		}(err)

		s.cb.Store(server.Reload)
		s.clientScript.Store(fmt.Sprintf(clientScript, Endpoint))
		select {
		case err := <-err:
			return err
		case <-ctx.Done():
			return nil
		}
	}
}

func (s *server) Client() template.HTML {
	client, _ := s.clientScript.Load().(string)
	return template.HTML(client)
}