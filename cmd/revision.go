package cmd

import (
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/databus23/helm-diff/diff"
	"github.com/databus23/helm-diff/manifest"
)

type revision struct {
	release string
	clientHolder
	detailedExitCode bool
	suppressedKinds  []string
	revisions        []string
	outputContext    int
	includeTests     bool
}

const revisionCmdLongUsage = `
This command compares the manifests details of a named release.

It can be used to compare the manifests of

 - lastest REVISION with specified REVISION
	$ helm diff revision [flags] RELEASE REVISION1
   Example:
	$ helm diff revision my-release 2

 - REVISION1 with REVISION2
	$ helm diff revision [flags] RELEASE REVISION1 REVISION2
   Example:
	$ helm diff revision my-release 2 3
`

func revisionCmd() *cobra.Command {
	diff := revision{}
	revisionCmd := &cobra.Command{
		Use:   "revision [flags] RELEASE REVISION1 [REVISION2]",
		Short: "Shows diff between revision's manifests",
		Long:  revisionCmdLongUsage,
		PreRun: func(*cobra.Command, []string) {
			expandTLSPaths()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// Suppress the command usage on error. See #77 for more info
			cmd.SilenceUsage = true

			if v, _ := cmd.Flags().GetBool("version"); v {
				fmt.Println(Version)
				return nil
			}

			switch {
			case len(args) < 2:
				return errors.New("Too few arguments to Command \"revision\".\nMinimum 2 arguments required: release name, revision")
			case len(args) > 3:
				return errors.New("Too many arguments to Command \"revision\".\nMaximum 3 arguments allowed: release name, revision1, revision2")
			}

			if q, _ := cmd.Flags().GetBool("suppress-secrets"); q {
				diff.suppressedKinds = append(diff.suppressedKinds, "Secret")
			}

			diff.release = args[0]
			diff.revisions = args[1:]
			diff.init()
			return diff.differentiate()
		},
	}

	revisionCmd.Flags().BoolP("suppress-secrets", "q", false, "suppress secrets in the output")
	revisionCmd.Flags().StringArrayVar(&diff.suppressedKinds, "suppress", []string{}, "allows suppression of the values listed in the diff output")
	revisionCmd.Flags().IntVarP(&diff.outputContext, "context", "C", -1, "output NUM lines of context around changes")
	revisionCmd.Flags().BoolVar(&diff.includeTests, "include-tests", false, "enable the diffing of the helm test hooks")
	revisionCmd.SuggestionsMinimumDistance = 1

	addCommonCmdOptions(revisionCmd.Flags())

	return revisionCmd
}

func (d *revision) differentiate() error {

	switch len(d.revisions) {
	case 1:
		releaseResponse, err := d.deployedRelease(d.release)

		if err != nil {
			return prettyError(err)
		}

		revision, _ := strconv.Atoi(d.revisions[0])
		revisionResponse, err := d.deployedReleaseRevision(d.release, revision)
		if err != nil {
			return prettyError(err)
		}

		diff.Manifests(
			manifest.ParseRelease(revisionResponse, d.includeTests),
			manifest.ParseRelease(releaseResponse, d.includeTests),
			d.suppressedKinds,
			d.outputContext,
			os.Stdout)

	case 2:
		revision1, _ := strconv.Atoi(d.revisions[0])
		revision2, _ := strconv.Atoi(d.revisions[1])
		if revision1 > revision2 {
			revision1, revision2 = revision2, revision1
		}

		revisionResponse1, err := d.deployedReleaseRevision(d.release, revision1)
		if err != nil {
			return prettyError(err)
		}

		revisionResponse2, err := d.deployedReleaseRevision(d.release, revision2)
		if err != nil {
			return prettyError(err)
		}

		seenAnyChanges := diff.Manifests(
			manifest.ParseRelease(revisionResponse1, d.includeTests),
			manifest.ParseRelease(revisionResponse2, d.includeTests),
			d.suppressedKinds,
			d.outputContext,
			os.Stdout)

		if d.detailedExitCode && seenAnyChanges {
			return Error{
				error: errors.New("identified at least one change, exiting with non-zero exit code (detailed-exitcode parameter enabled)"),
				Code:  2,
			}
		}

	default:
		return errors.New("Invalid Arguments")
	}

	return nil
}
