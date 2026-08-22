package main

import (
	"github.com/spf13/cobra"
)

func configureApplicationServer(app *application, root *cobra.Command) {
	root.Short = "Apply deterministic quality rules to Go software."
	root.Long = "goconduct composes deterministic Go quality plugins and serves their Protocol Buffer APIs."
	root.Args = cobra.NoArgs
	root.RunE = func(command *cobra.Command, _ []string) error {
		return app.Serve(command.Context())
	}
}
