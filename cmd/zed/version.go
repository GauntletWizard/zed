package main

import (
	"context"
	"fmt"
	"os"

	"github.com/authzed/authzed-go/pkg/responsemeta"
	v1 "github.com/authzed/authzed-go/proto/authzed/api/v1"
	"github.com/authzed/authzed-go/v1"
	"github.com/gookit/color"
	"github.com/jzelinskie/cobrautil"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"golang.org/x/term"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/authzed/zed/internal/storage"
)

func versionCmdFunc(cmd *cobra.Command, args []string) error {
	if !term.IsTerminal(int(os.Stdout.Fd())) {
		color.Disable()
	}

	includeRemoteVersion := cobrautil.MustGetBool(cmd, "include-remote-version")
	hasContext := false
	configStore, secretStore := defaultStorage()
	if includeRemoteVersion {
		_, err := storage.DefaultToken(
			cobrautil.MustGetString(cmd, "endpoint"),
			cobrautil.MustGetString(cmd, "token"),
			configStore,
			secretStore,
		)
		hasContext = err == nil
	}

	if hasContext && includeRemoteVersion {
		green := color.FgGreen.Render
		fmt.Print(green("client: "))
	}

	_, err := fmt.Println(cobrautil.UsageVersion("zed", cobrautil.MustGetBool(cmd, "include-deps")))
	if err != nil {
		return err
	}

	if hasContext && includeRemoteVersion {
		token, err := storage.DefaultToken(
			cobrautil.MustGetString(cmd, "endpoint"),
			cobrautil.MustGetString(cmd, "token"),
			configStore,
			secretStore,
		)
		if err != nil {
			return err
		}
		log.Trace().Interface("token", token).Send()

		client, err := authzed.NewClient(token.Endpoint, dialOptsFromFlags(cmd, token)...)
		if err != nil {
			return err
		}

		// NOTE: we ignore the error here, as it may be due to a schema not existing, or
		// the client being unable to connect, etc. We just treat all such cases as an unknown
		// version.
		var headerMD metadata.MD
		_, _ = client.ReadSchema(context.Background(), &v1.ReadSchemaRequest{}, grpc.Header(&headerMD))
		version := headerMD.Get(string(responsemeta.ServerVersion))

		blue := color.FgLightBlue.Render
		fmt.Print(blue("service: "))
		if len(version) == 1 {
			fmt.Println(version[0])
		} else {
			fmt.Println("(unknown)")
		}
	}

	return nil
}
