package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
)

type CLIOptions struct {
	Command       string
	ScenarioPath  string
	JSON          bool
	Events        bool
	Compact       bool
	ListScenarios bool
}

func ParseCLI(args []string) (CLIOptions, error) {
	options := CLIOptions{}
	if len(args) == 0 {
		return options, NewValidationError("command", "missing command")
	}
	options.Command = args[0]
	switch options.Command {
	case "run", "validate":
		if len(args) < 2 {
			return options, NewValidationError("scenario", "missing scenario path")
		}
		options.ScenarioPath = args[1]
		for _, arg := range args[2:] {
			switch arg {
			case "--json":
				options.JSON = true
			case "--events":
				options.Events = true
			case "--compact":
				options.Compact = true
			default:
				return options, NewValidationError("arg", "unknown argument "+arg)
			}
		}
	case "list", "--list":
		options.ListScenarios = true
	default:
		return options, NewValidationError("command", "unknown command "+options.Command)
	}
	return options, nil
}

func RunCLI(args []string) int {
	options, err := ParseCLI(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		PrintUsage(os.Stderr)
		return 2
	}
	if options.ListScenarios {
		PrintBuiltInScenarioList(os.Stdout)
		return 0
	}
	scenario, err := LoadScenario(options.ScenarioPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return 1
	}
	if err := ValidateScenario(scenario); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return 1
	}
	if options.Command == "validate" {
		fmt.Printf("ok %s\n", scenario.Name)
		return 0
	}
	ledger, err := RunScenario(scenario)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return 1
	}
	report := BuildReport(ledger, options.Events)
	if options.JSON || options.Compact {
		if options.Compact {
			err = WriteCompactReport(os.Stdout, report)
		} else {
			err = WriteReport(os.Stdout, report)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			return 1
		}
		return 0
	}
	PrintHumanReport(os.Stdout, report)
	return 0
}

func PrintUsage(out *os.File) {
	fmt.Fprintln(out, "usage:")
	fmt.Fprintln(out, "  ironcladdtl validate <scenario.json>")
	fmt.Fprintln(out, "  ironcladdtl run <scenario.json> [--json] [--events] [--compact]")
	fmt.Fprintln(out, "  ironcladdtl list")
}

func PrintHumanReport(out *os.File, report Report) {
	fmt.Fprintf(out, "IroncladDTL scenario=%s epoch=%d\n", report.Name, report.Epoch)
	fmt.Fprintf(out, "vaults=%d commitments=%d subs=%d locks=%d\n", len(report.Vaults), len(report.Commitments), len(report.Subs), len(report.Locks))
	fmt.Fprintf(out, "economic_claims=%d reserve=%d shortfall=%d\n", report.Metrics.TotalEconomicClaims, report.Metrics.TotalReserve, report.Metrics.EconomicShortfall)
	for _, warning := range report.Audit.Warnings {
		fmt.Fprintf(out, "warning: %s\n", warning)
	}
	if len(report.Rejected) > 0 {
		fmt.Fprintf(out, "rejected_operations=%d\n", len(report.Rejected))
	}
}

func PrintBuiltInScenarioList(out *os.File) {
	names := make([]string, 0, len(BuiltInScenarioNotes))
	for name := range BuiltInScenarioNotes {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		fmt.Fprintf(out, "%s\t%s\n", name, BuiltInScenarioNotes[name])
	}
}

func EncodeJSON(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(data)
}
