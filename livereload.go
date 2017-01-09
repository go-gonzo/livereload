// This package provide a Gonzo `stage` from https://github.com/omeid/livereload

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

var (
	Endpoint = "localhost:35729"
	Proto    = "http"
)

type Options struct {
	LiveCSS bool
}

type Server interface {
	Reload() gonzo.Stage
	Start() kargar.Action
	Client() func() template.HTML
}

func New(opt Options) Server {
	return &server{opt: opt}
}

type server struct {
	cb           atomic.Value
	clientScript atomic.Value
	opt          Options
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
					cb(path, s.opt.LiveCSS)
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
var lrs = document.createElement("script"); lrs.type = "text/javascript"; lrs.src = "%s://%s/livereload.js";
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
		s.clientScript.Store(fmt.Sprintf(clientScript, Proto, Endpoint))
		select {
		case err := <-err:
			return err
		case <-ctx.Done():
			return nil
		}
	}
}

func (s *server) Client() func() template.HTML {
	client, _ := s.clientScript.Load().(string)
	src := template.HTML(client)
	return func() template.HTML {
		return src
	}
}
