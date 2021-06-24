package main

import (
	"context"

	"github.com/authzed/authzed-go"
	v0 "github.com/authzed/authzed-go/arrakisapi/api"
	"github.com/jzelinskie/stringz"
	"github.com/open-policy-agent/opa/ast"
	opacmd "github.com/open-policy-agent/opa/cmd"
	"github.com/open-policy-agent/opa/rego"
	"github.com/open-policy-agent/opa/types"
	"github.com/spf13/cobra"
)

func registerExperimentCmd(rootCmd *cobra.Command) {
	rootCmd.AddCommand(experimentCmd)

	experimentCmd.AddCommand(opacmd.RootCommand)
	opacmd.RootCommand.Use = "opa"
	opacmd.RootCommand.PersistentPreRunE = opaPreRunCmdFunc
}

var experimentCmd = &cobra.Command{
	Use:               "experiment <subcommand>",
	Short:             "experimental functionality",
	PersistentPreRunE: persistentPreRunE,
}

func opaPreRunCmdFunc(cmd *cobra.Command, args []string) error {
	if err := persistentPreRunE(cmd, args); err != nil {
		return err
	}

	token, err := TokenFromFlags(cmd)
	if err != nil {
		return err
	}

	client, err := ClientFromFlags(cmd, token.Endpoint, token.Secret)
	if err != nil {
		return err
	}

	RegisterAuthzedBuiltins(token.System, client)

	return nil
}

func RegisterAuthzedBuiltins(system string, client *authzed.Client) {
	rego.RegisterBuiltin4(
		&rego.Function{
			Name:    "authzed.check",
			Decl:    types.NewFunction(types.Args(types.S, types.S, types.S, types.S), types.B),
			Memoize: false,
		},
		func(bctx rego.BuiltinContext, subjectTerm, relationTerm, objectTerm, zedtokenTerm *ast.Term) (*ast.Term, error) {
			var subjectStr, relation, objectStr, zedtoken string
			if err := ast.As(subjectTerm.Value, &subjectStr); err != nil {
				return nil, err
			}
			if err := ast.As(relationTerm.Value, &relation); err != nil {
				return nil, err
			}
			if err := ast.As(objectTerm.Value, &objectStr); err != nil {
				return nil, err
			}
			if err := ast.As(zedtokenTerm.Value, &zedtoken); err != nil {
				return nil, err
			}

			subjectNS, subjectID, subjectRel, err := parseSubject(subjectStr)
			if err != nil {
				return nil, err
			}

			var objectNS, objectID string
			err = stringz.SplitExact(objectStr, ":", &objectNS, &objectID)
			if err != nil {
				return nil, err
			}

			request := &v0.CheckRequest{
				TestUserset: &v0.ObjectAndRelation{
					Namespace: stringz.Join("/", system, objectNS),
					ObjectId:  objectID,
					Relation:  relation,
				},
				User: &v0.User{UserOneof: &v0.User_Userset{Userset: &v0.ObjectAndRelation{
					Namespace: stringz.Join("/", system, subjectNS),
					ObjectId:  subjectID,
					Relation:  subjectRel,
				}}},
			}

			if zedtoken != "" {
				request.AtRevision = &v0.Zookie{Token: zedtoken}
			}

			resp, err := client.Check(context.Background(), request)
			if err != nil {
				return nil, err
			}

			value, err := ast.InterfaceToValue(resp.IsMember)
			if err != nil {
				return nil, err
			}

			return ast.NewTerm(value), nil
		},
	)
}
