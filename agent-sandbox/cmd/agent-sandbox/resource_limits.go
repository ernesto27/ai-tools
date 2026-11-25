package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/ernesto27/ai-tools/agent-sandbox/config"
)

var (
	cpuPattern    = regexp.MustCompile(`^[0-9]+(?:\.[0-9]+)?(?:[eE][+-]?[0-9]+)?$`)
	memoryPattern = regexp.MustCompile(`^([0-9]+)(B|KiB|MiB|GiB|TiB)$`)
	memoryUnits   = map[string]int64{"B": 1, "KiB": 1 << 10, "MiB": 1 << 20, "GiB": 1 << 30, "TiB": 1 << 40}
)

func applyResourceLimits(cmd *cobra.Command, flags *runFlags, cfg config.Config, section *config.Section) error {
	cpus, memory, timeout := string(cfg.CPUs), cfg.Memory, cfg.Timeout
	if cpus == "" {
		cpus = "0"
	}
	if memory == "" {
		memory = "0"
	}
	if timeout == "" {
		timeout = "0"
	}
	if section != nil {
		if section.CPUs != nil {
			cpus = string(*section.CPUs)
		}
		if section.Memory != nil {
			memory = *section.Memory
		}
		if section.Timeout != nil {
			timeout = *section.Timeout
		}
	}
	if cmd.Flags().Changed(flagCPUs) {
		cpus = flags.cpus
	}
	if cmd.Flags().Changed(flagMemory) {
		memory = flags.memory
	}
	if cmd.Flags().Changed(flagTimeout) {
		timeout = flags.timeout
	}

	var err error
	if flags.NanoCPUs, err = parseCPUs(cpus); err != nil {
		return fmt.Errorf("--%s %w", flagCPUs, err)
	}
	if flags.MemoryBytes, err = parseMemory(memory); err != nil {
		return fmt.Errorf("--%s %w", flagMemory, err)
	}
	if flags.Timeout, err = parseTimeout(timeout); err != nil {
		return fmt.Errorf("--%s %w", flagTimeout, err)
	}
	return nil
}

func validateResourceLimit(key string, value json.RawMessage) error {
	if key == flagCPUs {
		if len(value) == 0 || value[0] != '-' && (value[0] < '0' || value[0] > '9') {
			return errors.New("must be a JSON number")
		}
		_, err := parseCPUs(string(value))
		return err
	}
	var input string
	if string(value) == "null" || json.Unmarshal(value, &input) != nil {
		return errors.New("must be a JSON string")
	}
	if key == flagMemory {
		_, err := parseMemory(input)
		return err
	}
	_, err := parseTimeout(input)
	return err
}

func parseCPUs(input string) (int64, error) {
	invalid := errors.New("must be a nonnegative number of cores representable in nanocores (for example 1.5)")
	if !cpuPattern.MatchString(input) {
		return 0, invalid
	}
	// Use exact arithmetic so rounding cannot turn a small positive limit
	// into an unlimited container or move a value across the int64 boundary.
	cores, ok := new(big.Rat).SetString(input)
	if !ok {
		return 0, invalid
	}
	nano := cores.Mul(cores, big.NewRat(1e9, 1))
	if !nano.IsInt() || !nano.Num().IsInt64() {
		return 0, invalid
	}
	value := nano.Num().Int64()
	// Docker's default 100 ms CPU period needs at least a 1 ms quota.
	// Tiny limits can round to zero and leave the container unrestricted.
	if value > 0 && value < 10_000_000 {
		return 0, errors.New("must be at least 0.01 cores, or 0 to clear the limit")
	}
	return value, nil
}

func parseMemory(input string) (int64, error) {
	if input == "0" {
		return 0, nil
	}
	parts := memoryPattern.FindStringSubmatch(input)
	if parts == nil {
		return 0, errors.New("must be a whole number with B, KiB, MiB, GiB or TiB units, or 0")
	}
	size, err := strconv.ParseInt(parts[1], 10, 64)
	unit := memoryUnits[parts[2]]
	if err != nil || size > math.MaxInt64/unit {
		return 0, errors.New("exceeds the maximum memory size")
	}
	bytes := size * unit
	// Docker requires at least 6 MiB for a nonzero memory limit.
	const minMemoryBytes = 6 << 20
	if bytes > 0 && bytes < minMemoryBytes {
		return 0, errors.New("must be at least 6 MiB, or 0 to clear the limit")
	}
	return bytes, nil
}

func parseTimeout(input string) (time.Duration, error) {
	duration, err := time.ParseDuration(input)
	if err != nil || strings.HasPrefix(input, "-") || duration == 0 && strings.ContainsAny(input, "123456789") {
		return 0, errors.New("must be a nonnegative duration (for example 30m), or 0")
	}
	return duration, nil
}
