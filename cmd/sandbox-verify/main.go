// Command sandbox-verify grades the engine's sandbox simulation against the
// canonical mechanics specifications. It runs the declarative scenario files
// under scenarios/sandbox against a headless world built from a real flattened
// game install and reports every check faithful / wrong / missing /
// cosmetic-gap. This is the local full-matrix driver; CI relies instead on the
// install-free golden gate in the sandboxverify package's tests.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/coreprime/kbot-engine/sandboxverify"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "sandbox-verify: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	taRoot := flag.String("ta-root", os.Getenv("TA_UNPACKED_PATH"), "flattened TA install (default $TA_UNPACKED_PATH)")
	takRoot := flag.String("tak-root", os.Getenv("TAK_UNPACKED_PATH"), "flattened TA:K install (default $TAK_UNPACKED_PATH)")
	jsonPath := flag.String("json", "", "write the machine-readable report to this path")
	strict := flag.Bool("strict", false, "exit non-zero when any check is not faithful")
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		args = []string{"scenarios/sandbox"}
	}

	var scenarios []*sandboxverify.Scenario
	for _, arg := range args {
		info, err := os.Stat(arg)
		if err != nil {
			return err
		}
		if info.IsDir() {
			batch, err := sandboxverify.LoadDir(arg)
			if err != nil {
				return err
			}
			scenarios = append(scenarios, batch...)
		} else {
			s, err := sandboxverify.Load(arg)
			if err != nil {
				return err
			}
			scenarios = append(scenarios, s)
		}
	}
	if len(scenarios) == 0 {
		return fmt.Errorf("no scenarios found")
	}

	runner := &sandboxverify.Runner{TARoot: *taRoot, TAKRoot: *takRoot}
	report := runner.Run(scenarios)
	fmt.Print(report.RenderTable())
	if *jsonPath != "" {
		if err := report.WriteJSON(*jsonPath); err != nil {
			return err
		}
		fmt.Printf("\nJSON report: %s\n", *jsonPath)
	}
	if *strict && report.AnyNonFaithful() {
		return fmt.Errorf("strict mode: non-faithful checks present")
	}
	return nil
}
